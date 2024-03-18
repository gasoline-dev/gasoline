import path from "node:path";
import { CliParsedArgs } from "../index.cli.js";
import { Config } from "./cli.config.js";
import { getDirFiles, getDirs } from "./cli.fs.js";

/**
 * Returns an array of resource dirs.
 *
 * @example
 * ```ts
 * ['gasoline/core-base-api']
 * ```
 */
async function getResourceDirs(
	resourceContainerDirs: string[],
): Promise<string[]> {
	const resourceDirs = await Promise.all(
		resourceContainerDirs.map(async (resourceContainerDir) => {
			const getDirsResult = await getDirs(resourceContainerDir);
			return getDirsResult.map((dir) => `${resourceContainerDir}/${dir}`);
		}),
	);
	return resourceDirs.flat();
}

type ResourceFiles = string[];

/**
 * Returns an array of resource files.
 *
 * @example
 * ```ts
 * ['gasoline/core-base-api/src/index.core.base.api.ts']
 * ```
 */
export async function getResourceFiles(
	resourceContainerDirs: ResourceContainerDirs,
): Promise<ResourceFiles> {
	const resourceDirs = await getResourceDirs(resourceContainerDirs);
	const resourceFiles = await Promise.all(
		resourceDirs.map(async (resourceDir) => {
			const getDirFilesResult = await getDirFiles(`${resourceDir}/src`, {
				fileRegexToMatch: /^[^.]+\.[^.]+\.[^.]+\.[^.]+\.[^.]+$/,
			});
			return getDirFilesResult.map((file) => `${resourceDir}/${file}`);
		}),
	);
	return resourceFiles.flat();
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
 * ['gasoline/core-base-api/index.core.base.api.ts']
 * // return:
 * ['core']
 * ```
 */
export function setResourceEntityGroups(resourceFiles: ResourceFiles) {
	const result: ResourceEntityGroups = [];
	for (const resourceFile of resourceFiles) {
		const group = path.basename(resourceFile).split(".")[1];
		if (!result.includes(group)) result.push(group);
	}
	return result;
}

type ResourceEntityGroupEntities = string[];

/**
 * Returns an array of resource entity group entities.
 *
 * @example
 * ```ts
 * // given resource files of:
 * ['gasoline/core-base-api/index.core.base.api.ts']
 * // return:
 * ['base']
 * ```
 */
export function setResourceEntityGroupEntities(resourceFiles: ResourceFiles) {
	const result: ResourceEntityGroupEntities = [];
	for (const resourceFile of resourceFiles) {
		const entity = path.basename(resourceFile).split(".")[2];
		if (!result.includes(entity)) result.push(entity);
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
	if (cliCommand === "add:cloudflare:worker:api:hono") return "api";
	throw new Error(
		`Resource descriptor cannot be set for CLI command: ${cliCommand}`,
	);
}
