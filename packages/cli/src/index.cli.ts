#!/usr/bin/env node
import { parseArgs } from "node:util";
import { runAddCommand } from "./commands/cli.add.js";
import { log, printVerboseLogs } from "./commons/cli.log.js";
import {
	ResourceExports,
	getResourceExports,
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
import {
	CloudflareKv,
	CloudflarePages,
	CloudflareWorker,
} from "@gasoline-dev/resources";

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

		const resourceExports = await getResourceExports(resourceIndexDistFiles);

		const resourceDirs = await getResourceDirs(resourceContainerDirs);

		const resourcePackageJsons = await getResourcePackageJsons(resourceDirs);

		const resourcePackageJsonNamesSet =
			setResourcePackageJsonNamesSet(resourcePackageJsons);

		const packageJsonNameToResourceIdMap = setPackageJsonNameToResourceIdMap(
			resourcePackageJsons,
			resourceExports,
		);

		const resourceDependencies = setResourceDependencies(
			resourcePackageJsons,
			packageJsonNameToResourceIdMap,
			resourcePackageJsonNamesSet,
		);

		const resourceMap = setResourceMap(resourceExports, resourceDependencies);

		console.log(resourceMap);

		const test = resourceMap.get("test");
		if (test && test.type === "cloudflare-pages") {
			console.log(test.exports.services);
		}

		// await deploy({}, resourceManifest);
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
	resourceExports: ResourceExports,
): PackageJsonNameToResourceIdMap {
	const result = new Map<string, string>();
	for (const [index, packageJson] of resourcePackageJsons.entries()) {
		result.set(packageJson.name, resourceExports[index].id);
	}
	return result;
}

type ResourceDependencies = Array<Array<string>>;

/**
 * Returns an array of resource dependencies.
 *
 * Resource dependencies are resources a resource depends on.
 * For example, resource `core:base:cloudflare-worker:12345`
 * might depend on `core:base:cloudflare-kv:12345`.
 *
 * @example
 * ```
 * // index 0 is core:base:cloudflare-worker:12345
 * // index 1 is core:base:cloudflare-kv:12345
 * // core:base:cloudflare-worker:12345 depends on
 * // core:base:cloudflare-kv:12345, while
 * // core:base:cloudflare-kv:12345 depends on nothing.
 * [ [ 'core:base:cloudflare-kv:12345' ], [] ]
 * ```
 */
function setResourceDependencies(
	resourcPackageJsons: ResourcePackageJsons,
	packageJsonNameToResourceIdMap: PackageJsonNameToResourceIdMap,
	resourcePackageJsonNamesSet: ResourcePackageJsonNamesSet,
): ResourceDependencies {
	const result: ResourceDependencies = [];
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

type ResourceMap = Map<
	string,
	(
		| {
				type: "cloudflare-kv";
				exports: CloudflareKv;
		  }
		| {
				type: "cloudflare-pages";
				exports: CloudflarePages;
		  }
		| {
				type: "cloudflare-worker";
				exports: CloudflareWorker;
		  }
	) & { dependsOn: Array<string> }
>;

function setResourceMap(
	resourceExports: ResourceExports,
	resourceDependencies: ResourceDependencies,
): ResourceMap {
	const result: ResourceMap = new Map();
	for (const [index, exports] of resourceExports.entries()) {
		result.set(exports.id, {
			//@ts-ignore
			// Resources should be validated before getting here.
			type: exports.id.split(":")[2],
			exports,
			dependsOn: resourceDependencies[index],
		});
	}
	return result;
}

async function deploy(prevResourceManifest: any, currResourceManifest: any) {}

await main();
