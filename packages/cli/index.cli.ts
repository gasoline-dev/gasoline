#!/usr/bin/env node
import { parseArgs, promisify } from "node:util";
import inquirer from "inquirer";
import fsPromises from "fs/promises";
import { downloadTemplate as downloadTemplateFromGitHub } from "giget";
import path from "node:path";
import { parseModule } from "magicast";
import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { Miniflare } from "miniflare";
import * as esbuild from "esbuild";
import { exec } from "node:child_process";
import { cwd } from "node:process";

await main();

async function main() {
	try {
		const options = {
			help: {
				type: "boolean",
				short: "h",
			},
			// Everything below this is for testing.
			dir: {
				type: "string",
			},
			entityGroup: {
				type: "string",
			},
		} as const;

		const parsedArgs = parseArgs({
			allowPositionals: true,
			options,
		});

		const helpMessage = `Usage:
gasoline [command] -> Run command

Commands:
 add:cloudflare:worker:api:empty Add Cloudflare Worker API

Options:
 --help, -h Print help`;

		if (parsedArgs.positionals?.[0]) {
			const command = parsedArgs.positionals[0];

			const commandDoesNotExistMessage = `Command "${command}" does not exist. Run "gasoline --help" to see available commands.`;

			if (command.includes("add:")) {
				const availableAddCommands = [
					"add:cloudflare:worker:api:empty",
					"add:cloudflare:worker:api:hono",
				];

				if (availableAddCommands.includes(command)) {
					await runAddCommand(command, parsedArgs.values);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else if (command === "dev") {
				await commandsRunDev();
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

async function runAddCommand(
	command: string,
	commandOptions: {
		[value: string]: boolean | string | undefined;
	},
) {
	type Config = {
		dirs?: [];
	};

	async function getConfig() {
		try {
			let result: Config = {};
			const configFile = "./gasoline.js";
			const configIsPresent = await isFilePresent(configFile);
			if (!configIsPresent) return result;
			const importedConfig = await import(path.join(cwd(), configFile));
			const configExport = importedConfig.default || module;
			result = {
				...configExport,
			};
			return result;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to get ./gasoline.js config");
		}
	}

	type ResourceContainerDirs = string[];

	type ResourceContainerOptions = {
		// An option passed to the CLI can technically be any of these types.
		commandDir?: undefined | boolean | string;
		configDirs?: string[];
	};

	/**
	 * Returns an array of resource container dirs. It returns
	 * an array so projects can have multiple resource container
	 * dirs if necessary.
	 *
	 * A resource container dir contains resource dirs.
	 *
	 * Most projects will have one resource container dir:
	 * `[./gasoline]`.
	 */
	function setResourceContainerDirs(options: ResourceContainerOptions = {}) {
		const { commandDir, configDirs } = options;
		let result: ResourceContainerDirs = [];
		if (typeof commandDir === "string") {
			result = [commandDir];
		} else if (configDirs) {
			result = configDirs;
		} else {
			result = ["./gasoline"];
		}
		return result;
	}

	type GetDirFilesOptions = {
		fileRegexToMatch?: RegExp;
	};

	async function getDirFiles(dir: string, options: GetDirFilesOptions = {}) {
		const { fileRegexToMatch = /.*/ } = options;
		try {
			const result = [];
			console.log(`Getting ${dir} directory files`);
			const entries = await fsPromises.readdir(dir, {
				withFileTypes: true,
			});
			for (const entry of entries) {
				if (!entry.isDirectory() && fileRegexToMatch.test(entry.name)) {
					result.push(entry.name);
				}
			}
			console.log(`Got ${dir} directory files`);
			return result;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to get ${dir} directory files`);
		}
	}

	async function isPathPresent(path: string) {
		try {
			await fsPromises.access(path);
			return true;
		} catch (error) {
			return false;
		}
	}

	async function isFilePresent(file: string) {
		try {
			console.log(`Checking if ${file} is present`);
			const pathIsPresent = await isPathPresent(file);
			if (pathIsPresent) {
				console.log(`${file} is present`);
				return true;
			}
			console.log(`${file} is not present`);
			return false;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to check if ${file} is present`);
		}
	}

	async function getDirs(dir: string) {
		try {
			console.log(`Getting ${dir} directories`);
			const entries = await fsPromises.readdir(dir, {
				withFileTypes: true,
			});
			const result: string[] = [];
			for (const entry of entries) {
				if (entry.isDirectory()) {
					result.push(entry.name);
				}
			}
			console.log(`Got ${dir} directories`);
			return result;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to get ${dir} directories`);
		}
	}

	type ResourceDirs = string[][];

	/**
	 * Returns an array of resource dir arrays.
	 *
	 * A resource dir contains resource files.
	 *
	 * @example
	 * ```ts
	 * const resourceContainerDirs = ['./gasoline']
	 * const result = await getResourceDirs(resourceContainerDirs)
	 * expect(result).toEqual([
	 * 	['core-base-api']
	 * ])
	 * ```
	 */
	async function getResourceDirs(resourceContainerDirs: string[]) {
		return await Promise.all(
			resourceContainerDirs.map((resourceContainerDir) =>
				getDirs(resourceContainerDir),
			),
		);
	}

	type ResourceContainerDirToResourceDirsMap = Map<string, string[]>;

	/**
	 * Returns a resource container dir to resource dirs map.
	 *
	 * @example
	 * ```ts
	 * const resourceContainerDirs = ['./gasoline']
	 * const resourceDirs = [
	 * 	['core-base-api']
	 * ]
	 * const result = setResourceContainerDirToResourceDirsMap(
	 * 	resourceContainerDirs,
	 * 	resourceDirs
	 * )
	 * expect(result).toEqual(new Map([
	 * 	['gasoline', 'core-base-api']
	 * ]))
	 * ```
	 */
	function setResourceContainerDirToResourceDirsMap(
		resourceContainerDirs: string[],
		resourceDirs: ResourceDirs,
	): ResourceContainerDirToResourceDirsMap {
		const result: ResourceContainerDirToResourceDirsMap = new Map();
		resourceContainerDirs.forEach((resourceContainerDir, index) => {
			result.set(resourceContainerDir, resourceDirs[index]);
		});
		return result;
	}

	type ResourceFiles = string[][];

	/**
	 * Returns an array of resource file arrays.
	 *
	 * Resource files exist in resource dirs.
	 *
	 * A resource file is the entry point of a resource.
	 *
	 * @example
	 * ```ts
	 * const resourceContainerDirToResourceDirsMap = new Map([
	 * 	['./gasoline', 'core-base-api']
	 * ])
	 * const result = await getResourceFiles(resourceContainerDirToResourceDirsMap)
	 * expect(result).toEqual([
	 * 	['index.core.base.api.ts']
	 * ])
	 * ```
	 */
	async function getResourceFiles(
		resourceContainerDirToResourceDirsMap: ResourceContainerDirToResourceDirsMap,
	): Promise<ResourceFiles> {
		const resourceContainerPromises: Promise<string[]>[] = [];

		for (const [
			resourceContainerDir,
			resourceDirs,
		] of resourceContainerDirToResourceDirsMap) {
			const dirPromises: Promise<string[]>[] = resourceDirs.map((resourceDir) =>
				getDirFiles(`${resourceContainerDir}/${resourceDir}`, {
					fileRegexToMatch: /^[^.]+\.[^.]+\.[^.]+\.[^.]+\.[^.]+$/,
				}),
			);
			resourceContainerPromises.push(
				Promise.all(dirPromises).then((filesArrays) => filesArrays.flat()),
			);
		}
		return Promise.all(resourceContainerPromises);
	}

	type ResourceContainerDirToResourceFilesMap = Map<string, string[]>;

	/**
	 * Returns a resource container dir to resource files map.
	 *
	 * @example
	 * ```ts
	 * const resourceContainerDirs = ['./gasoline']
	 * const resourceFiles = [
	 * 	['index.core.base.api.ts']
	 * ]
	 * const result = setResourceContainerDirToResourceFilesMap(
	 * 	resourceContainerDirs,
	 *	resourceFiles
	 * )
	 * expect(result).toEqual(new Map([
	 * 	['./gasoline', ['index.core.base.api.ts']]
	 * ]))
	 * ```
	 */
	async function setResourceContainerDirToResourceFilesMap(
		resourceContainerDirs: string[],
		resourceFiles: ResourceFiles,
	): Promise<ResourceContainerDirToResourceFilesMap> {
		const result: ResourceContainerDirToResourceFilesMap = new Map();
		resourceContainerDirs.forEach((resourceContainerDir, index) => {
			result.set(resourceContainerDir, resourceFiles[index]);
		});
		return result;
	}

	type ResourceContainerDirToResourceGroupsMap = Map<string, string[]>;

	/**
	 * Returns a resource container dir to resource groups map.
	 *
	 * @example
	 * ```
	 * const resourceContainerDirToResourceFilesMap = new Map([
	 * 	['./gasoline', ['index.core.base.api.ts']]
	 * ]
	 * const result = setResourceContainerDirToResourceGroupsMap(
	 * 	resourceContainerDirToResourceFilesMap
	 * )
	 * expect(result).toEqual(new Map([
	 * 	['./gasoline', ['core']]
	 * ]))
	 * ```
	 */
	function setResourceContainerDirToResourceGroupsMap(
		resourceContainerDirToResourceFilesMap: ResourceContainerDirToResourceFilesMap,
	): ResourceContainerDirToResourceGroupsMap {
		const result: ResourceContainerDirToResourceGroupsMap = new Map();
		for (const [
			resourceContainerDir,
			resourceFiles,
		] of resourceContainerDirToResourceFilesMap) {
			for (const resourceFile of resourceFiles) {
				const resourceGroup = resourceFile.split(".")[1];
				if (!result.has(resourceContainerDir)) {
					result.set(resourceContainerDir, []);
				}
				const resourceGroups = result.get(resourceContainerDir);
				if (!resourceGroups?.includes(resourceGroup)) {
					resourceGroups?.push(resourceGroup);
				}
			}
		}

		return result;
	}

	async function runSelectResourceContainerDirPrompt(
		resourceContainerDirs: ResourceContainerDirs,
	) {
		const { resourceContainerDir } = await inquirer.prompt([
			{
				type: "list",
				name: "resourceContainerDir",
				message: "Select resource container dir",
				choices: resourceContainerDirs,
			},
		]);
		return resourceContainerDir;
	}

	async function runSelectEntityGroupPrompt(resourceEntityGroups: string[]) {
		const { resourceEntityGroup } = await inquirer.prompt([
			{
				type: "list",
				name: "resourceEntityGroup",
				message: "Select resource entity group",
				choices: ["Add new", ...resourceEntityGroups],
			},
		]);
		return resourceEntityGroup;
	}

	async function runAddResourceEntityGroupPrompt() {
		const { resourceEntityGroup } = await inquirer.prompt([
			{
				type: "input",
				name: "resourceEntityGroup",
				message: "Enter resource entity group",
			},
		]);
		return resourceEntityGroup;
	}

	async function runResourceEntityGroupPrompt(resourceGroups: string[]) {
		let result = "";
		if (resourceGroups.length > 0) {
			result = await runSelectEntityGroupPrompt(resourceGroups);
		} else {
			result = await runAddResourceEntityGroupPrompt();
		}
		if (result === "Add new") {
			result = await runAddResourceEntityGroupPrompt();
		}
		return result;
	}

	type ResourceEntityGroupToEntitiesMap = Map<string, string[]>;

	/**
	 * Returns a resource entity group to entities map.
	 *
	 * @example
	 * ```ts
	 * const resourceContainerDirToResourceFilesMap = new Map([
	 * 	['./gasoline', ['index.core.base.api.ts']]
	 * ]
	 * const result = setResourceEntityGroupToEntitiesMap(
	 * 	resourceContainerDirToResourceFilesMap
	 * )
	 * expect(result).toEqual(new Map([
	 * 	['core', ['base']]
	 * ]))
	 * ```
	 */
	function setResourceEntityGroupToEntitiesMap(
		resourceContainerDirToResourceFilesMap: ResourceContainerDirToResourceFilesMap,
	) {
		const result: ResourceEntityGroupToEntitiesMap = new Map();
		for (const [_, resourceFiles] of resourceContainerDirToResourceFilesMap) {
			for (const resourceFile of resourceFiles) {
				const splitFile = resourceFile.split(".");
				const resourceEntityGroup = splitFile[1];
				const resourceEntity = splitFile[2];
				if (result.has(resourceEntityGroup)) {
					result.get(resourceEntityGroup)?.push(resourceEntity);
				} else {
					result.set(resourceEntityGroup, [resourceEntity]);
				}
			}
		}
		return result;
	}

	async function runSelectResourceEntityPrompt(
		resourceEntityGroupToEntitiesMap: ResourceEntityGroupToEntitiesMap,
		resourceEntityGroup: string,
	) {
		const { resourceEntity } = await inquirer.prompt([
			{
				type: "list",
				name: "resourceEntity",
				message: "Select resource entity",
				choices: [
					"Add new",
					...(resourceEntityGroupToEntitiesMap.get(resourceEntityGroup) || []),
				],
			},
		]);
		return resourceEntity;
	}

	async function runAddResourceEntity() {
		const { resourceEntity } = await inquirer.prompt([
			{
				type: "input",
				name: "resourceEntity",
				message: "Enter resource entity",
			},
		]);
		return resourceEntity;
	}

	async function runResourceEntityPrompt(
		resourceEntityGroupToEntitiesMap: ResourceEntityGroupToEntitiesMap,
		resourceEntityGroup: string,
	) {
		let result = await runSelectResourceEntityPrompt(
			resourceEntityGroupToEntitiesMap,
			resourceEntityGroup,
		);
		if (result === "Add new") {
			result = await runAddResourceEntity();
		}
		return result;
	}

	/**
	 * Returns a resource descriptor.
	 *
	 * A resource descriptor is the last part of a resource
	 * file. It describes what the resource is (e.g. `api` is
	 * the resource descriptor for `index.core.base.api.ts`).
	 */
	function setResourceDescriptor(command: string) {
		if (command === "add:cloudflare:worker:api:hono") return "api";
		throw new Error("Resource descriptor cannot be set for command");
	}

	async function getTemplate(src: string, targetDir: string) {
		try {
			console.log(`Downloading template ${src} to ${targetDir}`);
			await downloadTemplateFromGitHub(src, {
				dir: targetDir,
				forceClean: true,
			});
			console.log(`Downloaded template ${src} to ${targetDir}`);
		} catch (error) {
			console.error(error);
			throw new Error("Unable to download template");
		}
	}

	async function renameFile(oldPath: string, newPath: string) {
		try {
			console.log(`Renaming ${oldPath} to ${newPath}`);
			await fsPromises.rename(oldPath, newPath);
			console.log(`Renamed ${oldPath} to ${newPath}`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to rename ${oldPath} to ${newPath}`);
		}
	}

	type PackageManager = "npm" | "pnpm";

	async function getPackageManager(): Promise<PackageManager> {
		try {
			console.log("Getting package manager");
			const packageManagers: Array<PackageManager> = ["npm", "pnpm"];
			for (const packageManager of packageManagers) {
				switch (packageManager) {
					case "npm": {
						const isPackageLockJsonPresent =
							await isFilePresent("package-lock.json");
						if (isPackageLockJsonPresent) {
							console.log(`Got package manager -> ${packageManager}`);
							return packageManager;
						}
						break;
					}
					case "pnpm": {
						const isPnpmLockYamlPresent = await isFilePresent("pnpm-lock.yaml");
						if (isPnpmLockYamlPresent) {
							console.log(`Got package manager -> ${packageManager}`);
							return packageManager;
						}
						break;
					}
				}
			}
			throw new Error("No supported package manager found (npm or pnpm)");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to get package manager");
		}
	}

	async function installTemplatePackages(
		dir: string,
		packageManager: PackageManager,
	) {
		try {
			console.log("Installing packages");
			const promisifiedExec = promisify(exec);
			await promisifiedExec(`${packageManager} install`, {
				cwd: dir,
			});
			console.log("Installed packages");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to install packages");
		}
	}

	async function run() {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs({
			commandDir: commandOptions.dir,
			configDirs: config.dirs,
		});

		let selectedResourceContainerDir = resourceContainerDirs[0];
		if (resourceContainerDirs.length > 1) {
			selectedResourceContainerDir = await runSelectResourceContainerDirPrompt(
				resourceContainerDirs,
			);
		}

		const resourceDirs = await getResourceDirs([selectedResourceContainerDir]);

		const resourceContainerDirToResourceDirsMap =
			setResourceContainerDirToResourceDirsMap(
				[selectedResourceContainerDir],
				resourceDirs,
			);

		const resourceFiles = await getResourceFiles(
			resourceContainerDirToResourceDirsMap,
		);

		const resourceContainerDirToResourceFilesMap =
			await setResourceContainerDirToResourceFilesMap(
				[selectedResourceContainerDir],
				resourceFiles,
			);

		const resourceContainerDirToResourceGroupsMap =
			setResourceContainerDirToResourceGroupsMap(
				resourceContainerDirToResourceFilesMap,
			);

		const resourceEntityGroup = await runResourceEntityGroupPrompt([
			...(resourceContainerDirToResourceGroupsMap.get(
				selectedResourceContainerDir,
			) ?? []),
		]);

		const resourceEntityGroupToEntitiesMap =
			setResourceEntityGroupToEntitiesMap(
				resourceContainerDirToResourceFilesMap,
			);

		const resourceEntity = await runResourceEntityPrompt(
			resourceEntityGroupToEntitiesMap,
			resourceEntityGroup,
		);

		const resourceDescriptor = setResourceDescriptor(command);

		const templateSrc = `github:gasoline-dev/gasoline/templates/${command
			.replace("add:", "")
			.replace(/:/g, "-")}`;

		const templateTargetDir = path.join(
			selectedResourceContainerDir,
			`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
		);

		await getTemplate(templateSrc, templateTargetDir);

		await renameFile(
			path.join(templateTargetDir, "index.ts"),
			path.join(
				templateTargetDir,
				`index.${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.ts`,
			),
		);

		const packageManager = await getPackageManager();

		await installTemplatePackages(templateTargetDir, packageManager);

		console.log("Added template");
	}

	try {
		await run();
	} catch (error) {
		console.error(error);
		throw new Error("Unable to add template");
	}
}

async function commandsRunDev() {
	async function getGasolineDirFiles() {
		try {
			console.log("Reading gasoline directory");
			const result: string[] = [];
			async function recursiveRead(currentPath: string) {
				const entries = await fsPromises.readdir(currentPath, {
					withFileTypes: true,
				});
				for (const entry of entries) {
					const entryPath = path.join(currentPath, entry.name);
					if (entry.isDirectory()) {
						if (entry.name !== "node_modules" && entry.name !== ".store") {
							await recursiveRead(entryPath);
						}
					} else {
						if (entry.name.split(".").length === 4) {
							result.push(entry.name);
						}
					}
				}
			}
			await recursiveRead("./gasoline");
			console.log("Read gasoline directory");
			return result;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to read gasoline directory");
		}
	}

	const gasolineDirResourceFiles = await getGasolineDirFiles();

	const readGasolineDirResourceFilePromises: Promise<string>[] = [];
	for (const file of gasolineDirResourceFiles) {
		readGasolineDirResourceFilePromises.push(
			fsPromises.readFile(`./gasoline/${file}`, "utf-8"),
		);
	}

	const readGasolineDirResourceFilePromisesResult = await Promise.all(
		readGasolineDirResourceFilePromises,
	);

	type GasolineDirResourceFileToBody = {
		[resourceFile: string]: string;
	};

	const gasolineDirResourceFileToBody: GasolineDirResourceFileToBody = {};
	for (let i = 0; i < gasolineDirResourceFiles.length; i++) {
		gasolineDirResourceFileToBody[gasolineDirResourceFiles[i]] =
			readGasolineDirResourceFilePromisesResult[i];
	}

	type GasolineDirResourceFileToExportedConfigVar = {
		[resourceFile: string]: string | undefined;
	};

	const gasolineDirResourceFileToExportedConfigVarFilteredByType: GasolineDirResourceFileToExportedConfigVar =
		{};
	for (const file in gasolineDirResourceFileToBody) {
		const mod = parseModule(gasolineDirResourceFileToBody[file]);
		if (mod.exports) {
			for (const modExport in mod.exports) {
				// Assume this is a config export for now.
				if (
					mod.exports[modExport].id &&
					mod.exports[modExport].type &&
					// filter for cloudflare-worker for now.
					// this can be an optional filter later
					// when this function is extracted.
					mod.exports[modExport].type === "cloudflare-worker"
				) {
					gasolineDirResourceFileToExportedConfigVarFilteredByType[file] =
						modExport;
					break;
				}
			}
		}
	}

	console.log(gasolineDirResourceFileToExportedConfigVarFilteredByType);

	if (
		Object.keys(gasolineDirResourceFileToExportedConfigVarFilteredByType)
			.length > 0
	) {
		for (const resourceFile in gasolineDirResourceFileToExportedConfigVarFilteredByType) {
			console.log("running esbuild");
			await esbuild.build({
				entryPoints: [path.join(`./gasoline/${resourceFile}`)],
				bundle: true,
				format: "esm",
				outfile: `./gasoline/.store/cloudflare-worker-dev-bundles/${resourceFile.replace(
					".ts",
					".js",
				)}`,
				tsconfig: "./gasoline/tsconfig.json",
			});
			console.log("ran esbuild");
		}
	}

	console.log("Starting dev server");

	const app = new Hono();

	const mf = new Miniflare({
		modules: true,
		scriptPath:
			"./gasoline/.store/cloudflare-worker-dev-bundles/core.base.api.js",
	});
	app.get("/", async (c) => {
		const fetchRes = await mf.dispatchFetch("http://localhost:8787/");
		const text = await fetchRes.text();
		return c.text(text);
	});
	serve(app, (info) => {
		console.log(`Listening on http://localhost:${info.port}`);
	});
}
