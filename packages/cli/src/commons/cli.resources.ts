import path from "node:path";
import { CliParsedArgs } from "../index.cli.js";
import { Config } from "./cli.config.js";
import { getDirFiles, getDirs } from "./cli.fs.js";

export type ResourceDirs = string[];

/**
 * Returns an array of resource dirs.
 *
 * @example
 * ```ts
 * ['gasoline/core-base-api']
 * ```
 */
export async function getResourceDirs(
	resourceContainerDirs: string[],
): Promise<ResourceDirs> {
	const resourceDirs = await Promise.all(
		resourceContainerDirs.map(async (resourceContainerDir) => {
			const getDirsResult = await getDirs(resourceContainerDir);
			return getDirsResult.map((dir) => `${resourceContainerDir}/${dir}`);
		}),
	);
	return resourceDirs.flat();
}

export type ResourceIndexFiles = string[];

/**
 * @example
 * ```ts
 * ['gasoline/core-base-api/src/_core.base.api.index.ts']
 * ```
 */
export async function getResourceIndexFiles(
	resourceContainerDirs: ResourceContainerDirs,
): Promise<ResourceIndexFiles> {
	const resourceDirs = await getResourceDirs(resourceContainerDirs);
	const resourceIndexFiles = await Promise.all(
		resourceDirs.map(async (resourceDir) => {
			const getDirFilesResult = await getDirFiles(`${resourceDir}/src`, {
				fileRegexToMatch: /^_[^.]+\.[^.]+\.[^.]+\.[^.]+\.[^.]+$/,
			});
			return getDirFilesResult.map((file) => `${resourceDir}/src/${file}`);
		}),
	);
	return resourceIndexFiles.flat();
}

export type ResourceContainerDirs = string[];

/**
 * Returns an array of resource container dirs.
 *
 * It returns an array so projects can have multiple if necessary.
 *
 * Most projects will have one: `gasoline`.
 *
 * @example
 * ```ts
 * ['gasoline']
 * ```
 */
export function setResourceContainerDirs(
	cliResourceContainerDir: CliParsedArgs["values"]["resourceContainerDir"],
	configResourceContainerDirs: Config["resourceContainerDirs"],
) {
	let result: ResourceContainerDirs = [];
	if (cliResourceContainerDir) {
		result = [path.relative(process.cwd(), cliResourceContainerDir)];
	} else if (configResourceContainerDirs) {
		result = configResourceContainerDirs.map((dir) =>
			path.relative(process.cwd(), dir),
		);
	} else {
		result = ["gasoline"];
	}
	return result;
}

type ResourceEntityGroups = string[];

/**
 * Returns an array of resource entity groups.
 *
 * @example
 * ```ts
 * // given resource files of:
 * ['gasoline/core-base-api/_core.base.api.index.ts']
 * // return:
 * ['core']
 * ```
 */
export function setResourceEntityGroups(
	resourceIndexFiles: ResourceIndexFiles,
) {
	const result: ResourceEntityGroups = [];
	for (const resourceIndexFile of resourceIndexFiles) {
		if (!resourceIndexFile.includes(".dns.zone.")) {
			const group = path.basename(resourceIndexFile).split(".")[0];
			if (!result.includes(group)) result.push(group.replace("_", ""));
		}
	}
	return result;
}

type ResourceEntityGroupEntities = string[];

/**
 * @example
 * ```ts
 * // given resource files of:
 * ['gasoline/core-base-api/_core.base.api.index.ts']
 * // return:
 * ['base']
 * ```
 */
export function setResourceEntityGroupEntities(
	resourceIndexFiles: ResourceIndexFiles,
) {
	const result: ResourceEntityGroupEntities = [];
	for (const resourceIndexFile of resourceIndexFiles) {
		if (!resourceIndexFile.includes(".dns.zone.")) {
			const entity = path.basename(resourceIndexFile).split(".")[1];
			if (!result.includes(entity)) result.push(entity);
		}
	}
	return result;
}

/**
 * Returns a resource descriptor.
 *
 * A resource descriptor is the last part of a resource file. It
 * describes what the resource is. (e.g. `api` is the resource
 * descriptor for `gasoline/core-base-api/src/index.core.base.api.ts`).
 *
 * @example
 * ```ts
 * // given a cli command of:
 * "add:cloudflare:worker:api:hono"
 * // return:
 * "api"
 * ```
 */
export function setResourceDescriptor(cliCommand: string) {
	if (cliCommand === "add:cloudflare:dns:zone") return "zone";
	if (cliCommand === "add:cloudflare:kv") return "kv";
	if (cliCommand === "add:cloudflare:worker:api:empty") return "api";
	if (cliCommand === "add:cloudflare:worker:api:hono") return "api";
	throw new Error(
		`Resource descriptor cannot be set for CLI command: ${cliCommand}`,
	);
}
