#!/usr/bin/env node
import { parseArgs } from "node:util";
import { runAddCommand } from "./commands/cli.add.js";
import { log, printVerboseLogs } from "./commons/cli.log.js";
import {
	ResourceIndexDistFileExports,
	getResourceIndexDistFileExports,
	runDevCommand,
	setResourceIndexDistFiles,
} from "./commands/cli.dev.js";
import { runTurboPreBuildCommand } from "./commands/cli.turbo-pre-build.js";
import { runTurboPreDevCommand } from "./commands/cli.turbo-pre-dev.js";
import { runTurboInitCommand } from "./commands/cli.turbo-init.js";
import { getConfig } from "./commons/cli.config.js";
import {
	ResourceDirs,
	getResourceDirs,
	getResourceIndexFiles,
	setResourceContainerDirs,
} from "./commons/cli.resources.js";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { PackageJson } from "./commons/cli.packages.js";

const cliOptions = {
	help: {
		type: "boolean",
		short: "h",
	},
	verbose: {
		type: "boolean",
		short: "v",
	},
	dir: {
		type: "string",
	},
	entityGroup: {
		type: "string",
	},
	resourceContainerDir: {
		type: "string",
	},
} as const;

const cliParsedArgs = parseArgs({
	allowPositionals: true,
	options: cliOptions,
});

export type CliParsedArgs = typeof cliParsedArgs;

if (cliParsedArgs.values.verbose) printVerboseLogs();

async function main() {
	try {
		const helpMessage = `Usage:
gasoline [command] -> Run command

Commands:
 add:cloudflare:dns:zone         Add Cloudflare DNS zone
 add:cloudflare:kv               Add Cloudflare KV storage
 add:cloudflare:worker:api:empty Add Cloudflare Worker API
 add:cloudflare:worker:api:hono  Add Cloudflare Worker Hono API
 deploy                          Deploy system to the cloud

Options:
 --help, -h Print help`;

		if (cliParsedArgs.positionals?.[0]) {
			const cliCommand = cliParsedArgs.positionals[0];

			const commandDoesNotExistMessage = `Command "${cliCommand}" does not exist. Run "gasoline --help" to see available commands.`;

			if (cliCommand.includes("add:")) {
				const availableAddCommands = [
					"add:cloudflare:dns:zone",
					"add:cloudflare:kv",
					"add:cloudflare:worker:api:empty",
					"add:cloudflare:worker:api:hono",
				];

				if (availableAddCommands.includes(cliCommand)) {
					await runAddCommand(cliCommand, cliParsedArgs);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else if (cliCommand === "deploy") {
				await runDeployCommand();
			} else if (cliCommand === "dev") {
				await runDevCommand(cliParsedArgs);
			} else if (cliCommand.includes("turbo:")) {
				const availableTurboCommands = [
					"turbo:init",
					"turbo:pre-build",
					"turbo:pre-dev",
				];

				if (
					availableTurboCommands.includes(cliCommand) &&
					cliCommand === "turbo:init"
				) {
					await runTurboInitCommand(cliParsedArgs);
				} else if (
					availableTurboCommands.includes(cliCommand) &&
					cliCommand === "turbo:pre-dev"
				) {
					await runTurboPreDevCommand(cliParsedArgs);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else {
				console.log(commandDoesNotExistMessage);
			}
		} else {
			console.log(helpMessage);
		}
	} catch (error) {
		console.error(error);
	}
}

async function runDeployCommand() {
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceContainerDir = resourceContainerDirs[0];

		const resourceIndexFiles = await getResourceIndexFiles([
			resourceContainerDir,
		]);

		const resourceIndexDistFiles =
			setResourceIndexDistFiles(resourceIndexFiles);
		console.log(resourceIndexDistFiles);

		const resourceIndexDistFileExports = await getResourceIndexDistFileExports(
			resourceIndexDistFiles,
		);

		const resourceDirs = await getResourceDirs(resourceContainerDirs);

		const resourcePackageJsons = await getResourcePackageJsons(resourceDirs);

		const resourcePackageJsonNamesSet =
			setResourcePackageJsonNamesSet(resourcePackageJsons);
		console.log(resourcePackageJsonNamesSet);

		const packageJsonNameToResourceIdMap = setPackageJsonNameToResourceIdMap(
			resourcePackageJsons,
			resourceIndexDistFileExports,
		);
		console.log(packageJsonNameToResourceIdMap);

		const resourceInternalDependencies = setResourceInternalDependencies(
			resourcePackageJsons,
			packageJsonNameToResourceIdMap,
			resourcePackageJsonNamesSet,
		);

		const resourceManifest = setResourceManifest(
			resourceIndexDistFileExports,
			resourceInternalDependencies,
		);

		console.log(JSON.stringify(resourceManifest, null, 2));

		await deploy({}, resourceManifest);
	} catch (error) {
		log.error(error);
	}
}

type ResourcePackageJsons = Array<PackageJson>;

/**
 * Returns an array of parsed resource package.json files.
 *
 * @example
 * ```ts
 * [
 *   {
 *     "name": "core-base-api",
 *     ...
 *   }
 * ]
 * ```
 */
async function getResourcePackageJsons(
	resourceDirs: ResourceDirs,
): Promise<ResourcePackageJsons> {
	const resourcePackageJsons = await Promise.all(
		resourceDirs.map(async (resourceDir) => {
			const packageJson = await readFile(
				path.join(resourceDir, "package.json"),
				"utf-8",
			);
			return JSON.parse(packageJson);
		}),
	);
	return resourcePackageJsons;
}

type ResourcePackageJsonNamesSet = Set<string>;

/**
 * Returns a resource package.json names set.
 *
 * package.json names are derived from each resource's
 * package.json name property.
 *
 * @example
 * ```ts
 * { 'core-base-api' }
 * ```
 */
function setResourcePackageJsonNamesSet(
	resourcePackageJsons: ResourcePackageJsons,
): ResourcePackageJsonNamesSet {
	const result = new Set<string>();
	for (const packageJson of resourcePackageJsons) {
		result.add(packageJson.name);
	}
	return result;
}

type PackageJsonNameToResourceIdMap = Map<string, string>;

/**
 * Returns a `package.json` name to resource ID map.
 *
 * Resource relationships are managed via each resource's
 * `package.json`. For example, package `core-base-kv` might
 * be a dependency of package `core-base-api`. Therefore,
 * `core-base-kv` would exist in `core-base-api's` `package.json's`
 * _`dependencies`_.
 *
 * When `core-base-api's` `package.json` is processed and the
 * `core-base-kv` dependency is found, this map can look up `core-base-kv's` resource ID. Thus, establishing that resource
 * `core:base:cloudflare-kv:12345` is a dependency of
 * `core:base:cloudflare-worker:12345`.
 *
 * @example
 * ```ts
 * {
 *   'core-base-api' => 'core:base:cloudflare-worker:12345',
 *   'core-base-kv' => 'core:base:cloudflare-kv:12345'
 * }
 * ```
 */
function setPackageJsonNameToResourceIdMap(
	resourcePackageJsons: ResourcePackageJsons,
	resourceIndexDistFileExports: ResourceIndexDistFileExports,
): PackageJsonNameToResourceIdMap {
	const result = new Map<string, string>();
	for (const [index, packageJson] of resourcePackageJsons.entries()) {
		result.set(packageJson.name, resourceIndexDistFileExports[index].id);
	}
	return result;
}

type ResourceInternalDependencies = Array<Array<string>>;

function setResourceInternalDependencies(
	resourcPackageJsons: ResourcePackageJsons,
	packageJsonNameToResourceIdMap: PackageJsonNameToResourceIdMap,
	resourcePackageJsonNamesSet: ResourcePackageJsonNamesSet,
): ResourceInternalDependencies {
	const result: ResourceInternalDependencies = [];
	for (const [index, packageJson] of resourcPackageJsons.entries()) {
		const dependencies = Object.keys(packageJson.dependencies ?? {});
		const internalDependencies: Array<string> = [];
		for (const dependency of dependencies) {
			const resourceId = packageJsonNameToResourceIdMap.get(dependency);
			if (
				resourceId !== undefined &&
				resourcePackageJsonNamesSet.has(dependency)
			) {
				internalDependencies.push(resourceId);
			}
		}
		result.push(internalDependencies);
	}
	return result;
}

export type ResourceManifest = {
	entityGroups?: {
		[entityGroup: string]: {
			entities?: {
				[entity: string]: {
					resourceTypes?: CloudflareWorker;
				};
			};
		};
	};
};

type ResourceTypes = CloudflareKv & CloudflareWorker;

type CloudflareKv = {
	"cloudflare-kv"?: {
		resources: {
			[resourceId: string]: {
				config: {
					id: string;
					name: string;
				};
				dependsOn: Array<string>;
			};
		};
	};
};

type CloudflareWorker = {
	"cloudflare-worker"?: {
		resources: {
			[resourceId: string]: {
				config: {
					id: string;
					name: string;
					kv: Array<{
						binding: string;
					}>;
				};
				dependsOn: Array<string>;
			};
		};
	};
};

function setResourceManifest(
	resourceIndexDistFileExportedConfigs: ResourceIndexDistFileExports,
	resourceInternalDependencies: ResourceInternalDependencies,
): ResourceManifest {
	const result: ResourceManifest = {};
	result.entityGroups = {};
	for (const [
		index,
		config,
	] of resourceIndexDistFileExportedConfigs.entries()) {
		const splitId = config.id.split(":");
		const entityGroup = splitId[0];
		const entity = splitId[1];
		const resourceType = splitId[2] as keyof ResourceTypes;
		if (!result.entityGroups[entityGroup]) {
			result.entityGroups[entityGroup] = {};
		}
		if (!result.entityGroups[entityGroup].entities) {
			result.entityGroups[entityGroup].entities = {};
		}
		if (!result.entityGroups[entityGroup].entities![entity]) {
			result.entityGroups[entityGroup].entities![entity] = {};
		}
		if (!result.entityGroups[entityGroup].entities![entity].resourceTypes) {
			result.entityGroups[entityGroup].entities![entity].resourceTypes = {};
		}
		if (
			!result.entityGroups[entityGroup].entities![entity].resourceTypes![
				resourceType
			] &&
			resourceType === "cloudflare-worker"
		) {
			//
		}
	}
	return result;
}

async function deploy(
	prevResourceManifest: ResourceManifest,
	currResourceManifest: ResourceManifest,
) {
	const prevResourceToDirectDependenciesMap =
		setResourceToDirectDependenciesMap(prevResourceManifest);

	const currResourceToDirectDependenciesMap =
		setResourceToDirectDependenciesMap(currResourceManifest);

	const resourceToDirectDependenciesMap =
		setMergedPrevAndCurrResourceToDirectDependenciesMap(
			prevResourceToDirectDependenciesMap,
			currResourceToDirectDependenciesMap,
		);

	const resourceToUpstreamDependenciesMap =
		setResourceToUpstreamDependenciesMap(resourceToDirectDependenciesMap);

	console.log(resourceToUpstreamDependenciesMap);
}

type ResourceToDirectDependenciesMap = Map<string, Array<string>>;

/**
 * Returns a node to direct dependencies map.
 *
 * Direct dependencies are nodes the keyed node directly depends on.
 *
 * Example:
 *
 * ```ts
 * const jagCloud: JagCloud = {
 * 	Id: 'Jag',
 * 	EntityGroups: {
 * 		TestEntityGroup: {
 * 			Entities: {
 * 				TestService: {
 * 					NodeTypes: {
 * 						'AWS-APIGatewayHTTP-API': {
 * 							Regions: {
 * 								'US-East-1': {
 * 									Nodes: {
 * 										'Core::Cats::AWS-APIGatewayHTTP-API::US-East-1::Cats-V1-bcb6d99eb56a48e89040b92aab98e640':
 * 											{
 * 												Properties: {
 * 													Name: 'Jag-Core-Cats-Cats',
 * 													ProtocolType: 'HTTP',
 * 												},
 * 												DependsOn: [],
 * 											},
 * 									},
 * 								},
 * 							},
 * 						},
 * 						'AWS-APIGatewayHTTP-Stage': {
 * 							Regions: {
 * 								'US-East-1': {
 * 									Nodes: {
 * 										'Core::Cats::AWS-APIGatewayHTTP-Stage::US-East-1::Default-V1-824f1c89983b4da7b4da0c3a3b0cd050':
 * 											{
 * 												Properties: {
 * 													StageName: '$default',
 * 													AutoDeploy: true,
 * 												},
 * 												DependsOn: [
 * 													'Core::Cats::AWS-APIGatewayHTTP-API::US-East-1::Cats-V1-bcb6d99eb56a48e89040b92aab98e640',
 * 												],
 * 											},
 * 									},
 * 								},
 * 							},
 * 						},
 * 					},
 * 				},
 * 			},
 * 		},
 * 	},
 * };
 *
 * const result = nodeToDirectDependenciesMapSet(jagCloud);
 *
 * expect(result).toEqual({
 * 	'Core::Cats::AWS-APIGatewayHTTP-API::US-East-1::Cats-V1-bcb6d99eb56a48e89040b92aab98e640':
 * 		[],
 * 	'Core::Cats::AWS-APIGatewayHTTP-Stage::US-East-1::Default-V1-824f1c89983b4da7b4da0c3a3b0cd050':
 * 		[
 * 			'Core::Cats::AWS-APIGatewayHTTP-API::US-East-1::Cats-V1-bcb6d99eb56a48e89040b92aab98e640',
 * 		],
 * });
 * ```
 */
function setResourceToDirectDependenciesMap(
	resourceManifest: ResourceManifest,
): ResourceToDirectDependenciesMap {
	const result: ResourceToDirectDependenciesMap = new Map();
	/*
	if (resourceManifest.entityGroups) {
		for (const entityGroup in resourceManifest.entityGroups) {
			if (resourceManifest.entityGroups[entityGroup].entities) {
				for (const entity in resourceManifest.entityGroups[entityGroup]
					.entities) {
					if (
						resourceManifest.entityGroups[entityGroup].entities[entity]
							.resourceTypes
					) {
						for (const resourceType in resourceManifest.entityGroups[
							entityGroup
						].entities[entity].resourceTypes) {
							if (
								resourceManifest.entityGroups[entityGroup].entities[entity]
									.resourceTypes[resourceType]
							) {
								for (const region in resourceManifest.entityGroups[entityGroup]
									.entities[entity].resourceTypes[resourceType].regions) {
									if (
										resourceManifest.entityGroups[entityGroup].entities[entity]
											.resourceTypes[resourceType].regions[region].resources
									) {
										for (const resource in resourceManifest.entityGroups[
											entityGroup
										].entities[entity].resourceTypes[resourceType].regions[
											region
										].resources) {
											result.set(
												resource,
												resourceManifest.entityGroups[entityGroup].entities[
													entity
												].resourceTypes[resourceType].regions[region].resources[
													resource
												].dependsOn,
											);
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	*/
	return result;
}

/*
    prevNodeToDirectDependenciesMap may have nodes that don't
    exist in currNodeToDirectDependenciesMap if nodes were
    deleted. Those deleted nodes are accounted for by merging the
    two maps.
    */
function setMergedPrevAndCurrResourceToDirectDependenciesMap(
	prevResourceToDirectDependenciesMap: ResourceToDirectDependenciesMap,
	currResourceToDirectDependenciesMap: ResourceToDirectDependenciesMap,
): ResourceToDirectDependenciesMap {
	return new Map([
		...prevResourceToDirectDependenciesMap,
		...currResourceToDirectDependenciesMap,
	]);
}

type NodeToUpstreamDependenciesMap = {
	[node: string]: string[];
};

/**
 * Returns a node to upstream dependencies map.
 *
 * Upstream dependencies are nodes that exist ascendantly, excluding branches,
 * in the keyed node’s directed acyclic graph.
 *
 * Inspired by:
 * https://www.electricmonk.nl/docs/dependency_resolving_algorithm/dependency_resolving_algorithm.html
 *
 * Example:
 *
 * ```ts
 * const nodeToDirectDependenciesMap: NodeToDirectDependenciesMap = {
 * 	A: [],
 * 	B: ['A'],
 * 	C: ['B'],
 * 	D: ['A', 'B'],
 * 	X: [],
 * };
 *
 * const result = nodeToUpstreamDependenciesMapSet(
 * 	nodeToDirectDependenciesMap
 * );
 *
 * expect(result).toEqual({
 * 	A: [],
 * 	B: ['A'],
 * 	C: ['A', 'B'],
 * 	D: ['A', 'B'],
 * 	X: [],
 * });
 * ```
 */
function setResourceToUpstreamDependenciesMap(
	resourceToDirectDependenciesMap: ResourceToDirectDependenciesMap,
): NodeToUpstreamDependenciesMap {
	const result: NodeToUpstreamDependenciesMap = {};
	for (const [nodeToWalk] of resourceToDirectDependenciesMap) {
		const walkedNodes = walkNodeDependencies({
			nodeToDirectDependenciesMap: resourceToDirectDependenciesMap,
			nodeToWalk,
			walkedNodes: [],
			isFirstPass: true,
		});
		result[nodeToWalk] = walkedNodes;
	}
	return result;

	function walkNodeDependencies(options: {
		nodeToDirectDependenciesMap: ResourceToDirectDependenciesMap;
		nodeToWalk: string;
		walkedNodes: string[];
		isFirstPass?: boolean;
	}): string[] {
		const walkedNodes: string[] = [];
		const directDependencies = resourceToDirectDependenciesMap.get(
			options.nodeToWalk,
		);
		if (directDependencies) {
			for (const nodeToWalk of directDependencies) {
				// Prevent node from being walked more than once.
				if (!walkedNodes.includes(nodeToWalk)) {
					const newlyWalkedNodes = walkNodeDependencies({
						nodeToDirectDependenciesMap: resourceToDirectDependenciesMap,
						nodeToWalk,
						walkedNodes,
					});
					for (const node of newlyWalkedNodes) {
						// Prevent duplicate nodes.
						if (!walkedNodes.includes(node)) walkedNodes.push(node);
					}
				}
			}
		}
		// Prevent node from being added to own array of upstream dependencies.
		if (!options.isFirstPass) walkedNodes.push(options.nodeToWalk);
		return walkedNodes;
	}
}

type EndpointNodes = string[];

/**
 * Returns an array of endpoint nodes.
 *
 * Endpoint nodes are nodes that have dependencies and no dependents.
 *
 * They exist at the bottom of directed acyclic graphs.
 *
 * Example:
 *
 * ```ts
 * const nodeToUpstreamDependenciesMap: NodeToUpstreamDependenciesMap = {
 * 	A: [],
 * 	B: ['A'],
 * 	C: ['B'],
 * 	D: [],
 * 	X: [],
 * };
 *
 * const result = endpointNodesSet(nodeToUpstreamDependenciesMap);
 *
 * expect(result).toEqual(['C', 'D']);
 * ```
 */
function endpointResourcesSet(
	nodeToUpstreamDependenciesMap: NodeToUpstreamDependenciesMap,
): EndpointNodes {
	const result: EndpointNodes = [];
	for (const node of Object.keys(nodeToUpstreamDependenciesMap)) {
		if (
			nodeToUpstreamDependenciesMap[node].length > 0 &&
			!isNodeAnUpstreamDependencyOfAnyResource(
				nodeToUpstreamDependenciesMap,
				node,
			)
		)
			result.push(node);
	}
	return result;
}

/**
 * Returns true if node is an upstream dependency of another node, false if not.
 *
 * Example (true):
 *
 * ```ts
 * const nodeToUpstreamDependenciesMap: NodeToUpstreamDependenciesMap = {
 * 	A: [],
 * 	B: ['A'],
 * 	C: ['B'],
 * 	D: [],
 * 	X: [],
 * };
 *
 * const node = 'A';
 *
 * const result = isNodeAnUpstreamDependencyOfAnyNode(
 * 	nodeToUpstreamDependenciesMap,
 * 	node
 * );
 *
 * expect(result).toEqual(true);
 * ```
 *
 * Example (false):
 *
 * ```js
 * const nodeToUpstreamDependenciesMap: NodeToUpstreamDependenciesMap = {
 * 	A: [],
 * 	B: ['A'],
 * 	C: ['B'],
 * 	D: [],
 * 	X: [],
 * };
 *
 * const node = 'X';
 *
 * const result = isNodeAnUpstreamDependencyOfAnyNode(
 * 	nodeToUpstreamDependenciesMap,
 * 	node
 * );
 *
 * expect(result).toEqual(false);
 * ```
 */
function isNodeAnUpstreamDependencyOfAnyResource(
	nodeToUpstreamDependenciesMap: NodeToUpstreamDependenciesMap,
	node: string,
): boolean {
	let result = false;
	for (const nodeToCheck in nodeToUpstreamDependenciesMap) {
		if (nodeToUpstreamDependenciesMap[nodeToCheck].includes(node)) {
			result = true;
			break;
		}
	}
	return result;
}

await main();
