#!/usr/bin/env node
import { parseArgs, promisify } from "node:util";
import inquirer from "inquirer";
import { assign, createActor, fromPromise, setup, waitFor } from "xstate";
import fsPromises from "fs/promises";
import { downloadTemplate } from "giget";
import path from "node:path";
import { exec } from "node:child_process";
import { generateCode, loadFile, parseModule, writeFile } from "magicast";
import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { Miniflare } from "miniflare";
import * as esbuild from "esbuild";

await main();

async function main() {
	try {
		const options = {
			example: {
				type: "string",
			},
			help: {
				type: "boolean",
				short: "h",
			},
		} as const;

		const parsedArgs = parseArgs({
			allowPositionals: true,
			options,
		});

		if (parsedArgs.positionals?.[0]) {
			const command = parsedArgs.positionals[0];

			const isAddCommand = command.includes("add:") ? true : false;

			const availableAddCommands = [
				"add:cloudflare:worker:api:empty",
				"add:cloudflare:worker:api:hono",
			];

			if (isAddCommand) {
				if (availableAddCommands.includes(command)) {
					await runAddCommand(command);
				} else {
					console.log(`Command ${command} not found`);
				}
			}

			if (command === "dev") {
				await runDevCommand();
			}
		} else {
			logHelp();
		}
	} catch (error) {
		console.error(error);
	}
}

async function runAddCommand(commandUsed: string) {
	const gasolineDirectory = "./gasoline";
	const localTemplatesDirectory = "./gasoline/.store/templates";
	const templateName = commandUsed.replace("add:", "").replace(/:/g, "-");
	const localTemplateIndexPath = `./gasoline/.store/templates/${templateName}/index.ts`;
	const templateSource = `github:gasoline-dev/gasoline/templates/${templateName}`;

	const checkIfGasolineStoreTemplatesDirExists = fromPromise(async () => {
		try {
			console.log(`Checking if ${localTemplatesDirectory} directory exists`);
			const isGasolineStoreTemplatesDirPresent = await fsIsDirPresent(
				localTemplatesDirectory,
			);
			if (!isGasolineStoreTemplatesDirPresent) {
				console.log(`${localTemplatesDirectory} directory is not present`);
				return false;
			}
			console.log(`${localTemplatesDirectory} directory is present`);
			return true;
		} catch (error) {
			console.error(error);
			throw new Error(
				`Unable to check if ${localTemplatesDirectory} directory exists`,
			);
		}
	});

	const createGasolineStoreTemplatesDir = fromPromise(async () => {
		try {
			console.log(`Creating ${localTemplatesDirectory} directory`);
			await fsPromises.mkdir(localTemplatesDirectory, {
				recursive: true,
			});
			console.log(`Created ${localTemplatesDirectory} directory`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to create${localTemplatesDirectory} directory`);
		}
	});

	const getGasolineDirFiles = fromPromise(async () => {
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
			await recursiveRead(gasolineDirectory);
			console.log("Read gasoline directory");
			return result;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to read gasoline directory");
		}
	});

	type EntityGroupToEntities = {
		[entityGroup: string]: string[];
	};

	const setEntityGroupToEntities = fromPromise(
		async ({
			input,
		}: {
			input: { gasolineDirFiles: string[] };
		}) => {
			const entityGroupToEntities: EntityGroupToEntities = {};
			for (const file of input.gasolineDirFiles) {
				const splitFile = file.split(".");
				const [entityGroup, entityName] = splitFile;
				if (entityGroup in entityGroupToEntities) {
					entityGroupToEntities[entityGroup].push(entityName);
				} else {
					entityGroupToEntities[entityGroup] = [entityName];
				}
			}
			return entityGroupToEntities;
		},
	);

	type EntityGroup = string;

	const runEntityGroupPrompt = fromPromise(
		async ({
			input,
		}: {
			input: { entityGroups: string[] };
		}) => {
			const { entityGroup } = await inquirer.prompt([
				{
					type: "list",
					name: "entityGroup",
					message: "Select entity group",
					choices: ["Add new", ...input.entityGroups],
				},
			]);

			return entityGroup;
		},
	);

	const isEntityGroupSetToAddNew = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { entityGroup: string },
	) => {
		return params.entityGroup === "Add new";
	};

	const runAddEntityGroupPrompt = fromPromise(async () => {
		const { entityGroup } = await inquirer.prompt([
			{
				type: "input",
				name: "entityGroup",
				message: "Enter entity group",
			},
		]);

		return entityGroup;
	});

	const runEntityPrompt = fromPromise(
		async ({
			input,
		}: {
			input: {
				entityGroupToEntities: EntityGroupToEntities;
				entityGroup: EntityGroup;
			};
		}) => {
			const { entity } = await inquirer.prompt([
				{
					type: "list",
					name: "entity",
					message: "Select entity",
					choices: [
						"Add new",
						...input.entityGroupToEntities[input.entityGroup],
					],
				},
			]);

			return entity;
		},
	);

	const isEntitySetToAddNew = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { entity: string },
	) => {
		return params.entity === "Add new";
	};

	const runAddEntityPrompt = fromPromise(async () => {
		const { entity } = await inquirer.prompt([
			{
				type: "input",
				name: "entity",
				message: "Enter entity",
			},
		]);

		return entity;
	});

	const isGasolineStoreTemplatesDirPresent = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { isPresent: boolean },
	) => {
		return params.isPresent;
	};

	const downloadProvidedTemplate = fromPromise(async () => {
		try {
			console.log(`Downloading provided template ${templateSource}`);
			await downloadTemplate(templateSource, {
				dir: `${localTemplatesDirectory}/${templateName}`,
				forceClean: true,
			});
			console.log(`Downloaded provided template ${templateSource}`);
		} catch (error) {
			console.error(error);
			throw new Error("Unable to download provided template");
		}
	});

	const getDownloadedTemplatePackageJson = fromPromise(async () => {
		try {
			console.log("Getting downloaded template package.json");
			const packageJson = await fsPromises
				.readFile(
					path.join(localTemplatesDirectory, templateName, "package.json"),
					"utf-8",
				)
				.then(JSON.parse);
			console.log("Got downloaded template package.json");
			return packageJson;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to get downloaded template package.json");
		}
	});

	const getGasolineDirPackageJson = fromPromise(async () => {
		try {
			console.log("Getting gasoline directory package.json");
			const packageJson = await fsPromises
				.readFile(path.join(gasolineDirectory, "package.json"), "utf-8")
				.then(JSON.parse);
			console.log("Got gasoline directory package.json");
			return packageJson;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to get gasoline directory package.json");
		}
	});

	type PackageJson = {
		dependencies?: { [key: string]: string };
		devDependencies?: { [key: string]: string };
	};

	const setPackagesWithoutMajorVersionConflicts = fromPromise(
		async ({
			input,
		}: {
			input: {
				downloadedTemplatePackageJson: PackageJson;
				gasolineDirPackageJson: PackageJson;
			};
		}) => {
			const result: string[] = [];
			if (
				input.downloadedTemplatePackageJson.dependencies &&
				Object.keys(input.downloadedTemplatePackageJson.dependencies).length >
					0 &&
				input.gasolineDirPackageJson.dependencies &&
				Object.keys(input.gasolineDirPackageJson.dependencies).length > 0
			) {
				for (const downloadedTemplateDependency in input
					.downloadedTemplatePackageJson.dependencies) {
					if (
						input.gasolineDirPackageJson.dependencies[
							downloadedTemplateDependency
						] &&
						input.gasolineDirPackageJson.dependencies[
							downloadedTemplateDependency
						].split(".")[0] === downloadedTemplateDependency.split(".")[0]
					) {
						result.push(downloadedTemplateDependency);
					}
				}
			}
			return result;
		},
	);

	const setPackagesWithMajorVersionConflicts = fromPromise(
		async ({
			input,
		}: {
			input: {
				downloadedTemplatePackageJson: PackageJson;
				gasolineDirPackageJson: PackageJson;
			};
		}) => {
			const result: string[] = [];
			if (
				input.downloadedTemplatePackageJson.dependencies &&
				Object.keys(input.downloadedTemplatePackageJson.dependencies).length >
					0 &&
				input.gasolineDirPackageJson.dependencies &&
				Object.keys(input.gasolineDirPackageJson.dependencies).length > 0
			) {
				for (const downloadedTemplateDependency in input
					.downloadedTemplatePackageJson.dependencies) {
					if (
						input.gasolineDirPackageJson.dependencies[
							downloadedTemplateDependency
						] &&
						input.gasolineDirPackageJson.dependencies[
							downloadedTemplateDependency
						].split(".")[0] !== downloadedTemplateDependency.split(".")[0]
					) {
						result.push(downloadedTemplateDependency);
					}
				}
			}
			return result;
		},
	);

	const isThereAMajorVersionPackageConflict = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { packagesWithMajorVersionConflicts: string[] },
	) => {
		return params.packagesWithMajorVersionConflicts.length > 0;
	};

	type HowToResolveMajorVersionPackageConflictResult = {
		resolveMajorVersionPackageConflict: HowToResolveMajorVersionPackageConflictPromptAnswer;
	};

	type HowToResolveMajorVersionPackageConflictPromptAnswer =
		| "Update outdated"
		| "Use aliases"
		| "Cancel";

	const runHowToResolveMajorVersionPackageConflictPrompt = fromPromise(
		async () => {
			const { resolveMajorVersionPackageConflict } =
				await inquirer.prompt<HowToResolveMajorVersionPackageConflictResult>([
					{
						type: "list",
						name: "resolveMajorVersionPackageConflict",
						message:
							"There are major version package conflicts. What would you like to do?",
						choices: [
							"Update outdated",
							"Use aliases",
							"Cancel",
						] satisfies Array<HowToResolveMajorVersionPackageConflictPromptAnswer>,
						default: "Cancel",
					},
				]);

			return resolveMajorVersionPackageConflict;
		},
	);

	const isHowToResolveMajorVersionPackageConflictAnswerUpdate = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: {
			howToResolveMajorVersionPackageConflictAnswer: HowToResolveMajorVersionPackageConflictPromptAnswer;
		},
	) => {
		return (
			params.howToResolveMajorVersionPackageConflictAnswer === "Update outdated"
		);
	};

	const isHowToResolveMajorVersionPackageConflictAnswerAliases = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: {
			howToResolveMajorVersionPackageConflictAnswer: HowToResolveMajorVersionPackageConflictPromptAnswer;
		},
	) => {
		return (
			params.howToResolveMajorVersionPackageConflictAnswer === "Use aliases"
		);
	};

	const isHowToResolveMajorVersionPackageConflictAnswerCancel = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: {
			howToResolveMajorVersionPackageConflictAnswer: HowToResolveMajorVersionPackageConflictPromptAnswer;
		},
	) => {
		return params.howToResolveMajorVersionPackageConflictAnswer === "Cancel";
	};

	const getProjectPackageManager = fromPromise(async () => {
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
	});

	type PackageManager = "npm" | "pnpm";

	type PackagesWithMajorConflicts = string[];

	type PackagesWithoutMajorConflicts = string[];

	const installGasolinePackageJsonPackagesWithAliases = fromPromise(
		async ({
			input,
		}: {
			input: {
				downloadedTemplatePackageJson: PackageJson;
				gasolineDirPackageJson: PackageJson;
				packageManager: PackageManager;
				packagesWithMajorVersionConflicts: PackagesWithMajorConflicts;
				packagesWithoutMajorVersionConflicts: PackagesWithoutMajorConflicts;
			};
		}) => {
			const command: string[] = [input.packageManager];

			if (input.packageManager === "npm") {
				command.push("install");
			} else {
				command.push("add");
			}

			for (const packageWithoutMajorVersionConflicts of input.packagesWithoutMajorVersionConflicts) {
				command.push(packageWithoutMajorVersionConflicts);
			}

			for (const packageWithMajorVersionConflict of input.packagesWithMajorVersionConflicts) {
				if (
					input.gasolineDirPackageJson.dependencies?.[
						packageWithMajorVersionConflict
					] &&
					input.downloadedTemplatePackageJson.dependencies?.[
						packageWithMajorVersionConflict
					]
				) {
					const newPackageMajorVersion =
						input.downloadedTemplatePackageJson?.dependencies[
							packageWithMajorVersionConflict
						]
							.split(".")[0]
							.replace("^", "");

					const newPackageVersion =
						input.downloadedTemplatePackageJson?.dependencies[
							packageWithMajorVersionConflict
						];

					command.push(
						`${packageWithMajorVersionConflict}V${newPackageMajorVersion}@npm:${packageWithMajorVersionConflict}@${newPackageVersion}`,
					);
				}
			}

			command.push("--save");

			console.log(command.join(" "));

			try {
				console.log("Installing packages with aliases");
				//const promisifiedExec = promisify(exec);
				//await promisifiedExec(command.join(" "), {
				//cwd: gasolineDirectory,
				//});
				console.log("Installed packages with aliases");
			} catch (error) {
				console.error(error);
				throw new Error("Unable to install packages with aliases");
			}
		},
	);

	const replaceDownloadedTemplateImportsWithAliases = fromPromise(
		async ({
			input,
		}: {
			input: {
				downloadedTemplatePackageJson: PackageJson;
				gasolineDirPackageJson: PackageJson;
				packageManager: PackageManager;
				packagesWithMajorVersionConflicts: PackagesWithMajorConflicts;
				packagesWithoutMajorVersionConflicts: PackagesWithoutMajorConflicts;
			};
		}) => {
			try {
				console.log("Replacing downloaded template imports with aliases");

				const mod = await loadFile(localTemplateIndexPath);

				for (const packageWithMajorVersionConflict of input.packagesWithMajorVersionConflicts) {
					if (
						input.gasolineDirPackageJson.dependencies?.[
							packageWithMajorVersionConflict
						] &&
						input.downloadedTemplatePackageJson.dependencies?.[
							packageWithMajorVersionConflict
						]
					) {
						const newPackageMajorVersion =
							input.downloadedTemplatePackageJson?.dependencies[
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

				await writeFile(mod, localTemplateIndexPath);

				console.log("Replaced downloaded template imports with aliases");
			} catch (error) {
				console.error(error);
				throw new Error(
					"Unable to replace downloaded template imports with aliases",
				);
			}
		},
	);

	const copyDownloadedTemplateToGasolineDir = fromPromise(async () => {
		try {
			console.log("Copying downloaded template to gasoline directory");
			const excludedFiles = ["package.json", "tsconfig.json"];
			await fsPromises.cp(
				path.join(localTemplatesDirectory, templateName),
				gasolineDirectory,
				{
					recursive: true,
					filter: (src) => {
						const srcName = path.basename(src);
						return !excludedFiles.includes(srcName);
					},
				},
			);
			console.log("Copied downloaded template to gasoline directory");
		} catch (error) {
			console.error(error);
			throw new Error(
				"Unable to copy downloaded template to gasoline directory",
			);
		}
	});

	type Context = {
		gasolineDirFiles: string[];
		entityGroupToEntities: EntityGroupToEntities;
		entityGroup: EntityGroup;
		entity: string;
		commandUsed: string;
		downloadedTemplatePackageJson: undefined | PackageJson;
		gasolineDirPackageJson: undefined | PackageJson;
		packagesWithoutMajorVersionConflicts: PackagesWithMajorConflicts;
		packagesWithMajorVersionConflicts: PackagesWithoutMajorConflicts;
		packageManager: undefined | PackageManager;
		howToResolveMajorVersionPackageConflictPromptAnswer:
			| undefined
			| HowToResolveMajorVersionPackageConflictPromptAnswer;
	};

	const machine = setup({
		actors: {
			checkIfGasolineStoreTemplatesDirExists,
			createGasolineStoreTemplatesDir,
			getGasolineDirFiles,
			setEntityGroupToEntities,
			runEntityGroupPrompt,
			runAddEntityGroupPrompt,
			runEntityPrompt,
			runAddEntityPrompt,
			downloadProvidedTemplate,
			getDownloadedTemplatePackageJson,
			getGasolineDirPackageJson,
			setPackagesWithoutMajorVersionConflicts,
			setPackagesWithMajorVersionConflicts,
			runHowToResolveMajorVersionPackageConflictPrompt,
			getProjectPackageManager,
			installGasolinePackageJsonPackagesWithAliases,
			replaceDownloadedTemplateImportsWithAliases,
			copyDownloadedTemplateToGasolineDir,
		},
		guards: {
			isEntityGroupSetToAddNew,
			isEntitySetToAddNew,
			isGasolineStoreTemplatesDirPresent,
			isThereAMajorVersionPackageConflict,
			isHowToResolveMajorVersionPackageConflictAnswerUpdate,
			isHowToResolveMajorVersionPackageConflictAnswerAliases,
			isHowToResolveMajorVersionPackageConflictAnswerCancel,
		},
		types: {} as {
			context: Context;
			guards:
				| {
						type: "isGasolineStoreTemplatesDirPresent";
				  }
				| {
						type: "isThereAMajorVersionPackageConflict";
				  };
		},
	}).createMachine({
		id: "root",
		initial: "checkingIfGasolineStoreTemplatesDirExists",
		context: {
			gasolineDirFiles: [],
			entityGroupToEntities: {},
			entityGroup: "",
			entity: "",
			commandUsed,
			downloadedTemplatePackageJson: undefined,
			gasolineDirPackageJson: undefined,
			packagesWithoutMajorVersionConflicts: [],
			packagesWithMajorVersionConflicts: [],
			packageManager: undefined,
			howToResolveMajorVersionPackageConflictPromptAnswer: undefined,
		},
		states: {
			checkingIfGasolineStoreTemplatesDirExists: {
				invoke: {
					id: "checkingIfGasolineStoreTemplatesDirExists",
					src: "checkIfGasolineStoreTemplatesDirExists",
					onDone: [
						{
							target: "processingEntities",
							guard: {
								type: "isGasolineStoreTemplatesDirPresent",
								params: ({ event }) => ({
									isPresent: event.output,
								}),
							},
						},
						{
							target: "creatingGasolineStoreTemplatesDir",
						},
					],
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			creatingGasolineStoreTemplatesDir: {
				invoke: {
					id: "creatingGasolineStoreTemplatesDir",
					src: "createGasolineStoreTemplatesDir",
					onDone: {
						target: "downloadingTemplate",
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			processingEntities: {
				id: "processingEntities",
				initial: "gettingGasolineDirFiles",
				states: {
					gettingGasolineDirFiles: {
						invoke: {
							id: "gettingGasolineDirFiles",
							src: "getGasolineDirFiles",
							onDone: {
								target: "settingEntityGroupToEntities",
								actions: assign({
									gasolineDirFiles: ({ event }) => event.output,
								}),
							},
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					settingEntityGroupToEntities: {
						invoke: {
							id: "settingEntityGroupToEntities",
							src: "setEntityGroupToEntities",
							input: ({ context }) => ({
								gasolineDirFiles: context.gasolineDirFiles,
							}),
							onDone: {
								target: "runningEntityGroupPrompt",
								actions: assign({
									entityGroupToEntities: ({ event }) => event.output,
								}),
							},
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningEntityGroupPrompt: {
						invoke: {
							id: "runningEntityGroupPrompt",
							src: "runEntityGroupPrompt",
							input: ({ context }) => ({
								entityGroups: Object.keys(context.entityGroupToEntities),
							}),
							onDone: [
								{
									target: "runningAddEntityGroupPrompt",
									guard: {
										type: "isEntityGroupSetToAddNew",
										params: ({ event }) => ({
											entityGroup: event.output,
										}),
									},
									actions: assign({
										entityGroup: ({ event }) => event.output,
									}),
								},
								{
									target: "runningEntityPrompt",
									actions: assign({
										entityGroup: ({ event }) => event.output,
									}),
								},
							],
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningAddEntityGroupPrompt: {
						invoke: {
							id: "runningAddEntityGroupPrompt",
							src: "runAddEntityGroupPrompt",
							onDone: {
								target: "runningEntityPrompt",
								actions: assign({
									entityGroup: ({ event }) => event.output,
								}),
							},
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningEntityPrompt: {
						invoke: {
							id: "runningEntityPrompt",
							src: "runEntityPrompt",
							input: ({ context }) => ({
								entityGroupToEntities: context.entityGroupToEntities,
								entityGroup: context.entityGroup,
							}),
							onDone: [
								{
									target: "runningAddEntityPrompt",
									guard: {
										type: "isEntitySetToAddNew",
										params: ({ event }) => ({
											entity: event.output,
										}),
									},
									actions: assign({
										entity: ({ event }) => event.output,
									}),
								},
								{
									target: "#root.downloadingTemplate",
								},
							],
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningAddEntityPrompt: {
						invoke: {
							id: "runningAddEntityPrompt",
							src: "runAddEntityPrompt",
							onDone: {
								target: "#root.downloadingTemplate",
								actions: assign({
									entity: ({ event }) => event.output,
								}),
							},
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
				},
			},
			downloadingTemplate: {
				invoke: {
					id: "downloadingTemplate",
					src: "downloadProvidedTemplate",
					onDone: {
						target: "gettingDownloadedTemplatePackageJson",
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			gettingDownloadedTemplatePackageJson: {
				invoke: {
					id: "gettingDownloadedTemplatePackageJson",
					src: "getDownloadedTemplatePackageJson",
					onDone: {
						target: "gettingGasolineDirPackageJson",
						actions: assign({
							downloadedTemplatePackageJson: ({ event }) => event.output,
						}),
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			gettingGasolineDirPackageJson: {
				invoke: {
					id: "gettingGasolineDirPackageJson",
					src: "getGasolineDirPackageJson",
					onDone: {
						target: "settingPackagesWithoutMajorVersionConflicts",
						actions: assign({
							gasolineDirPackageJson: ({ event }) => event.output,
						}),
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			settingPackagesWithoutMajorVersionConflicts: {
				invoke: {
					id: "settingPackagesWithoutMajorVersionConflicts",
					src: "setPackagesWithoutMajorVersionConflicts",
					input: ({ context }) => ({
						downloadedTemplatePackageJson:
							context.downloadedTemplatePackageJson as PackageJson,
						gasolineDirPackageJson:
							context.gasolineDirPackageJson as PackageJson,
					}),
					onDone: {
						target: "settingPackagesWithMajorVersionConflicts",
						actions: assign({
							packagesWithoutMajorVersionConflicts: ({ event }) => event.output,
						}),
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			settingPackagesWithMajorVersionConflicts: {
				invoke: {
					id: "settingPackagesWithMajorVersionConflicts",
					src: "setPackagesWithMajorVersionConflicts",
					input: ({ context }) => ({
						downloadedTemplatePackageJson:
							context.downloadedTemplatePackageJson as PackageJson,
						gasolineDirPackageJson:
							context.gasolineDirPackageJson as PackageJson,
					}),
					onDone: [
						{
							target: "processingPackagesWithMajorVersionConflicts",
							guard: {
								type: "isThereAMajorVersionPackageConflict",
								params: ({ event }) => ({
									packagesWithMajorVersionConflicts: event.output,
								}),
							},
							actions: assign({
								packagesWithMajorVersionConflicts: ({ event }) => event.output,
							}),
						},
						{
							target: "ok",
						},
					],
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			processingPackagesWithMajorVersionConflicts: {
				id: "processingPackagesWithMajorVersionConflicts",
				initial: "runHowToResolvePrompt",
				states: {
					runHowToResolvePrompt: {
						invoke: {
							id: "runningHowToResolvePrompt",
							src: "runHowToResolveMajorVersionPackageConflictPrompt",
							onDone: [
								{
									target: "#root.ok",
									guard: {
										type: "isHowToResolveMajorVersionPackageConflictAnswerCancel",
										params: ({ event }) => ({
											howToResolveMajorVersionPackageConflictAnswer:
												event.output,
										}),
									},
								},
								{
									target: "gettingPackageManager",
									actions: assign({
										howToResolveMajorVersionPackageConflictPromptAnswer: ({
											event,
										}) => event.output,
									}),
								},
							],
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					gettingPackageManager: {
						invoke: {
							id: "gettingPackageManager",
							src: "getProjectPackageManager",
							onDone: [
								{
									target: "processingResolutionWithAliases",
									actions: assign({
										packageManager: ({ event }) => event.output,
									}),
									guard: {
										type: "isHowToResolveMajorVersionPackageConflictAnswerAliases",
										params: ({ context }) => ({
											howToResolveMajorVersionPackageConflictAnswer:
												context.howToResolveMajorVersionPackageConflictPromptAnswer as HowToResolveMajorVersionPackageConflictPromptAnswer,
										}),
									},
								},
							],
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					processingResolutionWithAliases: {
						id: "processingWithAlias",
						initial: "installingGasolinePackageJsonPackagesWithAliases",
						states: {
							installingGasolinePackageJsonPackagesWithAliases: {
								invoke: {
									id: "installingGasolinePackageJsonPackagesWithAliases",
									src: "installGasolinePackageJsonPackagesWithAliases",
									input: ({ context }) => ({
										downloadedTemplatePackageJson:
											context.downloadedTemplatePackageJson as PackageJson,
										gasolineDirPackageJson:
											context.gasolineDirPackageJson as PackageJson,
										packageManager: context.packageManager as PackageManager,
										packagesWithMajorVersionConflicts:
											context.packagesWithMajorVersionConflicts,
										packagesWithoutMajorVersionConflicts:
											context.packagesWithoutMajorVersionConflicts,
									}),
									onDone: {
										target: "replacingDownloadedTemplateImportsWithAliases",
									},
									onError: {
										target: "#root.err",
										actions: ({ context, event }) => console.error(event),
									},
								},
							},
							replacingDownloadedTemplateImportsWithAliases: {
								invoke: {
									id: "replacingDownloadedTemplateImportsWithAliases",
									src: "replaceDownloadedTemplateImportsWithAliases",
									input: ({ context }) => ({
										downloadedTemplatePackageJson:
											context.downloadedTemplatePackageJson as PackageJson,
										gasolineDirPackageJson:
											context.gasolineDirPackageJson as PackageJson,
										packageManager: context.packageManager as PackageManager,
										packagesWithMajorVersionConflicts:
											context.packagesWithMajorVersionConflicts,
										packagesWithoutMajorVersionConflicts:
											context.packagesWithoutMajorVersionConflicts,
									}),
									onDone: {
										target: "#root.copyingDownloadedTemplateToGasolineDir",
									},
									onError: {
										target: "#root.err",
										actions: ({ context, event }) => console.error(event),
									},
								},
							},
						},
					},
				},
			},
			copyingDownloadedTemplateToGasolineDir: {
				invoke: {
					id: "copyingDownloadedTemplateToGasolineDir",
					src: "copyDownloadedTemplateToGasolineDir",
					onDone: {
						target: "ok",
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			ok: {
				type: "final",
			},
			err: {
				type: "final",
			},
		},
	});

	const actor = createActor(machine).start();

	const snapshot = await waitFor(
		actor,
		(snapshot) => snapshot.matches("ok") || snapshot.matches("err"),
		{
			timeout: 3600_000,
		},
	);

	if (snapshot.value === "err") {
		throw new Error("Unable to add template");
	}
}

function logHelp() {
	console.log(`Usage:
gasoline [command] -> Run command
gas [command] -> Run command

Commands:
 add:cloudflare:worker:api:empty Add Cloudflare Worker API

Options:
 --help, -h Print help`);
}

async function fsIsDirPresent(directory: string) {
	try {
		await fsPromises.access(directory);
		return true;
	} catch (error) {
		return false;
	}
}

async function runDevCommand() {
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
