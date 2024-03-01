#!/usr/bin/env node
import { parseArgs, promisify } from "node:util";
import inquirer from "inquirer";
import fsPromises from "fs/promises";
import { downloadTemplate as downloadTemplateFromGitHub } from "giget";
import path from "node:path";
import { loadFile, parseModule, writeFile } from "magicast";
import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { Miniflare } from "miniflare";
import * as esbuild from "esbuild";
import { exec } from "node:child_process";

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
					await commandsRunAdd(command, parsedArgs.values);
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

async function commandsRunAdd(
	command: string,
	commandOptions: {
		[value: string]: boolean | string | undefined;
	},
) {
	async function fsAccess(dir: string) {
		try {
			await fsPromises.access(dir);
			return true;
		} catch (error) {
			return false;
		}
	}

	async function fsCreateDir(dir: string) {
		try {
			console.log(`Creating ${dir} directory`);
			await fsPromises.mkdir(dir, {
				recursive: true,
			});
			console.log(`Created ${dir} directory`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to create${dir} directory`);
		}
	}

	async function fsEnsureDirIsPresent(dir: string) {
		const isDirPresent = await fsIsDirPresent(dir);
		if (!isDirPresent) await fsCreateDir(dir);
	}

	type FsGetDirFilesOptions = {
		fileRegexToMatch?: RegExp;
	};

	async function fsReadDirFiles(
		dir: string,
		options: FsGetDirFilesOptions = {},
	) {
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

	async function fsIsDirPresent(dir: string) {
		try {
			console.log(`Checking if ${dir} directory is present`);
			const isDirPresent = await fsAccess(dir);
			if (!isDirPresent) {
				console.log(`${dir} directory is not present`);
				return false;
			}
			console.log(`${dir} directory is present`);
			return true;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to check if ${dir} directory exists`);
		}
	}

	type ResourcesEntityGroupToEntitiesMap = Map<string, string[]>;

	function resourcesSetEntityGroupToEntitiesMap(
		files: string[],
	): ResourcesEntityGroupToEntitiesMap {
		const result: ResourcesEntityGroupToEntitiesMap = new Map();
		for (const file of files) {
			const splitFile = file.split(".");
			const [resourceEntityGroup, resourceEntity] = splitFile;
			if (result.has(resourceEntityGroup)) {
				result.get(resourceEntityGroup)?.push(resourceEntity);
			} else {
				result.set(resourceEntityGroup, [resourceEntity]);
			}
		}
		return result;
	}

	async function resourcesRunSelectEntityGroupPrompt(
		resourceEntityGroups: string[],
	) {
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

	async function resourcesRunAddEntityGroupPrompt() {
		const { resourceEntityGroup } = await inquirer.prompt([
			{
				type: "input",
				name: "resourceEntityGroup",
				message: "Enter resource entity group",
			},
		]);
		return resourceEntityGroup;
	}

	async function resourcesRunEntityGroupPrompt(resourceGroups: string[]) {
		let result = "";
		if (resourceGroups.length > 0) {
			result = await resourcesRunSelectEntityGroupPrompt(resourceGroups);
		} else {
			result = await resourcesRunAddEntityGroupPrompt();
		}
		if (result === "Add new") {
			result = await resourcesRunAddEntityGroupPrompt();
		}
		return result;
	}

	async function resourcesRunSelectEntityPrompt(
		resourceEntityGroupToEntitiesMap: ResourcesEntityGroupToEntitiesMap,
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

	async function resourcesRunAddEntity() {
		const { resourceEntity } = await inquirer.prompt([
			{
				type: "input",
				name: "resourceEntity",
				message: "Enter resource entity",
			},
		]);
		return resourceEntity;
	}

	async function resourcesRunEntityPrompt(
		resourceEntityGroupToEntitiesMap: ResourcesEntityGroupToEntitiesMap,
		resourceEntityGroup: string,
	) {
		let result = await resourcesRunSelectEntityPrompt(
			resourceEntityGroupToEntitiesMap,
			resourceEntityGroup,
		);
		if (result === "Add new") {
			result = await resourcesRunAddEntity();
		}
		return result;
	}

	async function templatesGet(
		templateDir: string,
		templateName: string,
		templateSrc: string,
	) {
		try {
			console.log(
				`Downloading template ${templateSrc} to ${templateDir}/${templateName}`,
			);
			await downloadTemplateFromGitHub(templateSrc, {
				dir: `${templateDir}/${templateName}`,
				forceClean: true,
			});
			console.log(
				`Downloaded template ${templateSrc} to ${templateDir}/${templateName}`,
			);
		} catch (error) {
			console.error(error);
			throw new Error("Unable to download template");
		}
	}

	async function fsReadJsonFile(file: string) {
		try {
			console.log(`Reading ${file}`);
			const packageJson = await fsPromises
				.readFile(file, "utf-8")
				.then(JSON.parse);
			console.log(`Read ${file}`);
			return packageJson;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to read ${file}`);
		}
	}

	type PackageJson = {
		dependencies?: { [key: string]: string };
		devDependencies?: { [key: string]: string };
	};

	function templatesSetPackagesWithoutMajorVersionConflicts(
		base: PackageJson,
		compare: PackageJson,
	) {
		const result: string[] = [];
		if (
			base.dependencies &&
			Object.keys(base.dependencies).length > 0 &&
			compare.dependencies &&
			Object.keys(compare.dependencies).length > 0
		) {
			for (const downloadedTemplateDependency in base.dependencies) {
				if (
					compare.dependencies[downloadedTemplateDependency] &&
					compare.dependencies[downloadedTemplateDependency].split(".")[0] ===
						downloadedTemplateDependency.split(".")[0]
				) {
					result.push(downloadedTemplateDependency);
				}
			}
		}
		return result;
	}

	function templatesSetPackagesWithMajorVersionConflicts(
		base: PackageJson,
		compare: PackageJson,
	) {
		const result: string[] = [];
		if (
			base.dependencies &&
			Object.keys(base.dependencies).length > 0 &&
			compare.dependencies &&
			Object.keys(compare.dependencies).length > 0
		) {
			for (const downloadedTemplateDependency in base.dependencies) {
				if (
					compare.dependencies[downloadedTemplateDependency] &&
					compare.dependencies[downloadedTemplateDependency].split(".")[0] !==
						downloadedTemplateDependency.split(".")[0]
				) {
					result.push(downloadedTemplateDependency);
				}
			}
		}
		return result;
	}

	type TemplatesRunHowToResolvePackagesWithMajorVersionConflictPromptResult =
		| "Update outdated"
		| "Use aliases";

	async function templatesRunHowToResolvePackagesWithMajorVersionConflictPrompt() {
		const { result } = await inquirer.prompt<{
			result: TemplatesRunHowToResolvePackagesWithMajorVersionConflictPromptResult;
		}>([
			{
				type: "list",
				name: "result",
				message:
					"There are major version package conflicts. What would you like to do?",
				choices: [
					"Update outdated",
					"Use aliases",
				] satisfies Array<TemplatesRunHowToResolvePackagesWithMajorVersionConflictPromptResult>,
				default: "Update outdated",
			},
		]);
		return result;
	}

	function templatesSetPackageAliases(
		templatePackageJson: PackageJson,
		templatePackagesWithMajorVersionConflicts: string[],
		gasolineDirPackageJson: PackageJson,
	) {
		const result: string[] = [];
		for (const packageWithMajorVersionConflict of templatePackagesWithMajorVersionConflicts) {
			if (
				gasolineDirPackageJson.dependencies?.[
					packageWithMajorVersionConflict
				] &&
				templatePackageJson.dependencies?.[packageWithMajorVersionConflict]
			) {
				const newPackageMajorVersion = templatePackageJson?.dependencies[
					packageWithMajorVersionConflict
				]
					.split(".")[0]
					.replace("^", "");

				const newPackageVersion =
					templatePackageJson?.dependencies[packageWithMajorVersionConflict];

				result.push(
					`${packageWithMajorVersionConflict}V${newPackageMajorVersion}@npm:${packageWithMajorVersionConflict}@${newPackageVersion}`,
				);
			}
		}
		return result;
	}

	async function templatesReplaceImportsWithAliases(options: {
		gasolineDirPackageJson: PackageJson;
		templateIndexPath: string;
		templatePackageJson: PackageJson;
		templatePackagesWithMajorVersionConflicts: string[];
	}) {
		try {
			console.log("Replacing template imports with aliases");

			const mod = await loadFile(options.templateIndexPath);

			for (const packageWithMajorVersionConflict of options.templatePackagesWithMajorVersionConflicts) {
				if (
					options.gasolineDirPackageJson.dependencies?.[
						packageWithMajorVersionConflict
					] &&
					options.templatePackageJson.dependencies?.[
						packageWithMajorVersionConflict
					]
				) {
					const newPackageMajorVersion =
						options.templatePackageJson?.dependencies[
							packageWithMajorVersionConflict
						]
							.split(".")[0]
							.replace("^", "");

					for (const item of mod.imports.$items) {
						if (item.from === packageWithMajorVersionConflict) {
							mod.imports.$add({
								from: `${packageWithMajorVersionConflict}V${newPackageMajorVersion}`,
								imported: item.local,
							});
						}
					}
				}
			}

			await writeFile(mod, options.templateIndexPath);

			console.log("Replaced template imports with aliases");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to replace template imports with aliases");
		}
	}

	type FsCopyDirOptions = {
		excludedFiles?: string[];
	};

	async function fsCopyDirFiles(
		dir: string,
		targetDir: string,
		options: FsCopyDirOptions = {},
	) {
		const { excludedFiles = [] } = options;
		try {
			console.log(`Copying ${dir} to ${targetDir}`);
			await fsPromises.cp(dir, targetDir, {
				recursive: true,
				filter:
					excludedFiles.length > 0
						? (src) => {
								const srcName = path.basename(src);
								return !excludedFiles.includes(srcName);
						  }
						: undefined,
			});
			console.log(`Copied ${dir} to ${targetDir}`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to copy ${dir} to ${targetDir}`);
		}
	}

	type PackageManager = "npm" | "pnpm";

	async function packageManagerSet(): Promise<PackageManager> {
		console.log("Getting project package manager");
		const isRootPackageJsonPresent = await fsPromises
			.access("package.json")
			.then(
				() => true,
				() => false,
			);

		if (isRootPackageJsonPresent) {
			const packageJson = JSON.parse(
				await fsPromises.readFile("package.json", "utf8"),
			);
			if ("workspaces" in packageJson) {
				return "npm";
			}
		}

		const isPnpm = await fsPromises.access("./pnpm-workspace.yaml").then(
			() => true,
			() => false,
		);

		if (isPnpm) return "pnpm";

		throw new Error("Unable to get project package manager");
	}

	function packageManagerSetInstallCommand(
		packageManager: PackageManager,
		packagesToInstall: string[],
	) {
		const command: string[] = [];
		switch (packageManager) {
			case "npm":
				command.push("npm");
				command.push("install");
				break;
			case "pnpm":
				command.push("pnpm");
				command.push("add");
				break;
			default: {
				const never: never = packageManager;
				throw new Error(`Unexpected package manager -> ${packageManager}`);
			}
		}
		command.push(...packagesToInstall);
		command.push("--save");
		return command.join(" ");
	}

	async function fsRenameFile(oldPath: string, newPath: string) {
		try {
			console.log(`Renaming ${oldPath} to ${newPath}`);
			await fsPromises.rename(oldPath, newPath);
			console.log(`Renamed ${oldPath} to ${newPath}`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to rename ${oldPath} to ${newPath}`);
		}
	}

	function resourcesSetDescriptor(command: string) {
		if (command === "add:cloudflare:worker:api:hono") return "api";
		throw new Error("Resource descriptor cannot be set for command");
	}

	async function packageManagerRunInstallCommand(command: string, dir: string) {
		try {
			console.log("Installing packages");
			const promisifiedExec = promisify(exec);
			await promisifiedExec(command, {
				cwd: dir,
			});
			console.log("Installed packages");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to install packages");
		}
	}

	async function run() {
		const gasolineDir =
			typeof commandOptions.dir === "string"
				? commandOptions.dir
				: "./gasoline";
		const templateDir = path.join(gasolineDir, ".store/templates");
		const templateName = command.replace("add:", "").replace(/:/g, "-");
		const templateIndexPath = path.join(
			templateDir,
			`${templateName}/index.ts`,
		);
		const templateSrc = `github:gasoline-dev/gasoline/templates/${templateName}`;

		await fsEnsureDirIsPresent(gasolineDir);

		const gasolineDirFiles = await fsReadDirFiles(gasolineDir, {
			fileRegexToMatch: /^[^.]+\.[^.]+\.[^.]+\.[^.]+$/,
		});

		const resourceEntityGroupToEntitiesMap =
			resourcesSetEntityGroupToEntitiesMap(gasolineDirFiles);

		const resourceEntityGroup = await resourcesRunEntityGroupPrompt([
			...resourceEntityGroupToEntitiesMap.keys(),
		]);

		const resourceEntity = await resourcesRunEntityPrompt(
			resourceEntityGroupToEntitiesMap,
			resourceEntityGroup,
		);

		await fsEnsureDirIsPresent(templateDir);

		await templatesGet(templateDir, templateName, templateSrc);

		const [templatePackageJson, gasolineDirPackageJson] = await Promise.all([
			fsReadJsonFile(path.join(templateDir, templateName, "package.json")),
			fsReadJsonFile(path.join(gasolineDir, "package.json")),
		]);

		const templatePackagesWithoutMajorVersionConflicts =
			templatesSetPackagesWithoutMajorVersionConflicts(
				templatePackageJson,
				gasolineDirPackageJson,
			);

		const templatePackagesWithMajorVersionConflicts =
			templatesSetPackagesWithMajorVersionConflicts(
				templatePackageJson,
				gasolineDirPackageJson,
			);

		const packagesToInstall: string[] = [];

		if (templatePackagesWithMajorVersionConflicts.length > 0) {
			const howToResolve =
				await templatesRunHowToResolvePackagesWithMajorVersionConflictPrompt();
			switch (howToResolve) {
				case "Use aliases":
					packagesToInstall.push(
						...templatesSetPackageAliases(
							templatePackageJson,
							templatePackagesWithMajorVersionConflicts,
							gasolineDirPackageJson,
						),
					);

					await templatesReplaceImportsWithAliases({
						gasolineDirPackageJson,
						templateIndexPath,
						templatePackageJson,
						templatePackagesWithMajorVersionConflicts,
					});

					break;
				case "Update outdated":
					// TODO
					break;
				default: {
					const never: never = howToResolve;
					throw new Error(`Unexpected answer -> ${howToResolve}`);
				}
			}
		}

		packagesToInstall.push(...templatePackagesWithoutMajorVersionConflicts);

		await fsCopyDirFiles(path.join(templateDir, templateName), gasolineDir, {
			excludedFiles: ["package.json", "tsconfig.json"],
		});

		const resourceDescriptor = resourcesSetDescriptor(command);

		await fsRenameFile(
			path.join(gasolineDir, "index.ts"),
			path.join(
				gasolineDir,
				`${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.ts`,
			),
		);

		const packageManager = await packageManagerSet();

		const installCommand = packageManagerSetInstallCommand(
			packageManager,
			packagesToInstall,
		);

		await packageManagerRunInstallCommand(installCommand, gasolineDir);
	}

	await run();
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
