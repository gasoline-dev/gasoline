#!/usr/bin/env node
import { parseArgs } from "node:util";
import { runAddCommand } from "./commands/cli.add.js";
import { log, printVerboseLogs } from "./commons/cli.log.js";
import {
	getResourceIndexDistFileExportedConfigs,
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

		const resourceIndexDistFileExportedConfigs =
			await getResourceIndexDistFileExportedConfigs(resourceIndexDistFiles);

		console.log(resourceIndexDistFileExportedConfigs);

		const resourceDirs = await getResourceDirs(resourceContainerDirs);

		console.log(resourceDirs);

		const resourcePackageJsons = await getResourcePackageJsons(resourceDirs);

		console.log(resourcePackageJsons);

		const resourcePackageNamesSet =
			setResourcePackageNamesSet(resourcePackageJsons);

		console.log(resourcePackageNamesSet);

		const packageJsonNameToResourceIdMap = setPackageJsonNameToResourceIdMap(
			resourcePackageJsons,
			resourceIndexDistFileExportedConfigs,
		);

		console.log(packageJsonNameToResourceIdMap);

		const resourceInternalDependencies = setResourceInternalDependencies(
			resourcePackageJsons,
			packageJsonNameToResourceIdMap,
			resourcePackageNamesSet,
		);

		console.log(resourceInternalDependencies);

		const resourceManifest = setResourceManifest(
			resourceIndexDistFileExportedConfigs,
			resourceInternalDependencies,
		);

		console.log(JSON.stringify(resourceManifest, null, 2));
	} catch (error) {
		log.error(error);
	}
}

type ResourcePackageJsons = Array<PackageJson>;

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

type ResourcePackageNamesSet = Set<string>;

function setResourcePackageNamesSet(
	resourcePackageJsons: ResourcePackageJsons,
): ResourcePackageNamesSet {
	const result = new Set<string>();
	for (const packageJson of resourcePackageJsons) {
		result.add(packageJson.name);
	}
	return result;
}

type PackageJsonNameToResourceIdMap = Map<string, string>;

function setPackageJsonNameToResourceIdMap(
	resourcePackageJsons: ResourcePackageJsons,
	resourceIndexDistFileExportedConfigs: any,
): PackageJsonNameToResourceIdMap {
	const result = new Map<string, string>();
	for (const [index, packageJson] of resourcePackageJsons.entries()) {
		result.set(
			packageJson.name,
			resourceIndexDistFileExportedConfigs[index].id,
		);
	}
	return result;
}

type ResourceInternalDependencies = Array<Array<string>>;

function setResourceInternalDependencies(
	resourcPackageJsons: ResourcePackageJsons,
	packageJsonNameToResourceIdMap: PackageJsonNameToResourceIdMap,
	resourcePackageNamesSet: ResourcePackageNamesSet,
): ResourceInternalDependencies {
	const result: ResourceInternalDependencies = [];
	for (const [index, packageJson] of resourcPackageJsons.entries()) {
		const dependencies = Object.keys(packageJson.dependencies ?? {});
		const internalDependencies: Array<string> = [];
		for (const dependency of dependencies) {
			if (
				resourcePackageNamesSet.has(dependency) &&
				packageJsonNameToResourceIdMap.has(dependency)
			) {
				internalDependencies.push(
					packageJsonNameToResourceIdMap.get(dependency),
				);
			}
		}
		result.push(internalDependencies);
	}
	return result;
}

type ResourceManifest = any;

function setResourceManifest(
	resourceIndexDistFileExportedConfigs: any,
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
		const resourceType = splitId[2];
		const region = "NONE";
		if (!result.entityGroups[entityGroup]) {
			result.entityGroups[entityGroup] = {};
		}
		if (!result.entityGroups[entityGroup].entities) {
			result.entityGroups[entityGroup].entities = {};
		}
		if (!result.entityGroups[entityGroup].entities[entity]) {
			result.entityGroups[entityGroup].entities[entity] = {};
		}
		if (!result.entityGroups[entityGroup].entities[entity].resourceTypes) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes = {};
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			]
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			] = {};
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions = {};
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region]
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region] = {};
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources = {};
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources[config.id]
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources[config.id] = {};
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources[config.id].config
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources[config.id].config = config;
		}
		if (
			!result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources[config.id].dependsOn
		) {
			result.entityGroups[entityGroup].entities[entity].resourceTypes[
				resourceType
			].regions[region].resources[config.id].dependsOn =
				resourceInternalDependencies[index];
		}
	}
	return result;
}

await main();
