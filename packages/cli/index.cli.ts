#!/usr/bin/env node
import { parseArgs, promisify } from "node:util";
import inquirer from "inquirer";
import { assign, createActor, fromPromise, setup, waitFor } from "xstate";
import fsPromises from "fs/promises";
import { downloadTemplate } from "giget";
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
					await runAddCommand(command);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else if (command === "dev") {
				await runDevCommand();
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

async function runAddCommand(commandUsed: string) {
	const gasolineDir = "./gasoline";
	const localTemplatesDir = "./gasoline/.store/templates";
	const templateName = commandUsed.replace("add:", "").replace(/:/g, "-");
	const localTemplateIndexPath = `./gasoline/.store/templates/${templateName}/index.ts`;
	const templateSource = `github:gasoline-dev/gasoline/templates/${templateName}`;

	const checkIfGasolineStoreTemplatesDirExists = fromPromise(async () => {
		return await fsIsDirPresent(localTemplatesDir);
	});

	const createGasolineStoreTemplatesDir = fromPromise(async () => {
		return await fsCreateDir(localTemplatesDir);
	});

	const getGasolineDirFiles = fromPromise(async () => {
		return await fsGetDirFiles(gasolineDir, {
			fileRegexToMatch: /^[^.]+\.[^.]+\.[^.]+\.[^.]+$/,
		});
	});

	const setResourceEntityGroupToEntitiesMap = fromPromise(
		async ({
			input,
		}: {
			input: { gasolineDirFiles: string[] };
		}) => {
			return resourcesSetEntityGroupToEntitiesMap(input.gasolineDirFiles);
		},
	);

	type ResourceEntityGroup = string;

	const runSelectResourceEntityGroupPrompt = fromPromise(
		async ({
			input,
		}: {
			input: { resourceEntityGroups: string[] };
		}) => {
			return await promptsRunSelectResourceEntityGroup(
				input.resourceEntityGroups,
			);
		},
	);

	const isResourceEntityGroupSetToAddNew = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { resourceEntityGroup: string },
	) => {
		return params.resourceEntityGroup === "Add new";
	};

	const runAddResourceEntityGroupPrompt = fromPromise(async () => {
		return await promptsRunAddResourceEntityGroupPrompt();
	});

	const runSelectResourceEntityPrompt = fromPromise(
		async ({
			input,
		}: {
			input: {
				resourceEntityGroupToEntitiesMap: ResourcesEntityGroupToEntitiesMap;
				resourceEntityGroup: ResourceEntityGroup;
			};
		}) => {
			return await promptsRunSelectResourceEntity(
				input.resourceEntityGroupToEntitiesMap,
				input.resourceEntityGroup,
			);
		},
	);

	const isResourceEntitySetToAddNew = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { resourceEntity: string },
	) => {
		return params.resourceEntity === "Add new";
	};

	const runAddResourceEntityPrompt = fromPromise(async () => {
		return await promptsAddResourceEntity();
	});

	const isGasolineStoreTemplatesDirPresent = (
		// biome-ignore lint/suspicious/noExplicitAny: <explanation>
		_: any,
		params: { isPresent: boolean },
	) => {
		return params.isPresent;
	};

	const downloadTemplate = fromPromise(async () => {
		return await templatesDownload({
			src: templateSource,
			downloadDir: localTemplatesDir,
			templateName,
		});
	});

	const getDownloadedTemplatePackageJson = fromPromise(async () => {
		return await templatesGetDownloadedPackageJson(
			localTemplatesDir,
			templateName,
		);
	});

	const getGasolineDirPackageJson = fromPromise(async () => {
		return gasolineDirGetPackageJson(gasolineDir);
	});

	const setDownloadedTemplatePackageJsonPackagesWithoutMajorVersionConflicts =
		fromPromise(
			async ({
				input,
			}: {
				input: {
					downloadedTemplatePackageJson: PackageJson;
					gasolineDirPackageJson: PackageJson;
				};
			}) => {
				return templatesSetDownloadedPackageJsonPackagesWithoutMajorVersionConflicts(
					input.downloadedTemplatePackageJson,
					input.gasolineDirPackageJson,
				);
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
				const promisifiedExec = promisify(exec);
				await promisifiedExec(command.join(" "), {
					cwd: gasolineDir,
				});
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
				path.join(localTemplatesDir, templateName),
				gasolineDir,
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
		resourceEntityGroupToEntitiesMap: ResourcesEntityGroupToEntitiesMap;
		resourceEntityGroup: ResourceEntityGroup;
		resourceEntity: string;
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
			setResourceEntityGroupToEntitiesMap,
			runSelectResourceEntityGroupPrompt,
			runAddResourceEntityGroupPrompt,
			runSelectResourceEntityPrompt,
			runAddResourceEntityPrompt,
			downloadTemplate,
			getDownloadedTemplatePackageJson,
			getGasolineDirPackageJson,
			setDownloadedTemplatePackageJsonPackagesWithoutMajorVersionConflicts,
			setPackagesWithMajorVersionConflicts,
			runHowToResolveMajorVersionPackageConflictPrompt,
			getProjectPackageManager,
			installGasolinePackageJsonPackagesWithAliases,
			replaceDownloadedTemplateImportsWithAliases,
			copyDownloadedTemplateToGasolineDir,
		},
		guards: {
			isResourceEntityGroupSetToAddNew,
			isResourceEntitySetToAddNew,
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
			resourceEntityGroupToEntitiesMap: new Map(),
			resourceEntityGroup: "",
			resourceEntity: "",
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
							target: "processingResourceEntities",
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
			processingResourceEntities: {
				id: "processingResourceEntities",
				initial: "gettingGasolineDirFiles",
				states: {
					gettingGasolineDirFiles: {
						invoke: {
							id: "gettingGasolineDirFiles",
							src: "getGasolineDirFiles",
							onDone: {
								target: "settingResourceEntityGroupToEntitiesMap",
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
					settingResourceEntityGroupToEntitiesMap: {
						invoke: {
							id: "settingResourceEntityGroupToEntitiesMap",
							src: "setResourceEntityGroupToEntitiesMap",
							input: ({ context }) => ({
								gasolineDirFiles: context.gasolineDirFiles,
							}),
							onDone: {
								target: "runningSelectResourceEntityGroupPrompt",
								actions: assign({
									resourceEntityGroupToEntitiesMap: ({ event }) => event.output,
								}),
							},
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningSelectResourceEntityGroupPrompt: {
						invoke: {
							id: "runningSelectResourceEntityGroupPrompt",
							src: "runSelectResourceEntityGroupPrompt",
							input: ({ context }) => ({
								resourceEntityGroups: [
									...context.resourceEntityGroupToEntitiesMap.keys(),
								],
							}),
							onDone: [
								{
									target: "runningAddResourceEntityGroupPrompt",
									guard: {
										type: "isResourceEntityGroupSetToAddNew",
										params: ({ event }) => ({
											resourceEntityGroup: event.output,
										}),
									},
									actions: assign({
										resourceEntityGroup: ({ event }) => event.output,
									}),
								},
								{
									target: "runningSetResourceEntityPrompt",
									actions: assign({
										resourceEntityGroup: ({ event }) => event.output,
									}),
								},
							],
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningAddResourceEntityGroupPrompt: {
						invoke: {
							id: "runningAddResourceEntityGroupPrompt",
							src: "runAddResourceEntityGroupPrompt",
							onDone: {
								target: "runningSetResourceEntityPrompt",
								actions: assign({
									resourceEntityGroup: ({ event }) => event.output,
								}),
							},
							onError: {
								target: "#root.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningSetResourceEntityPrompt: {
						invoke: {
							id: "runningSetResourceEntityPrompt",
							src: "runSelectResourceEntityPrompt",
							input: ({ context }) => ({
								resourceEntityGroupToEntitiesMap:
									context.resourceEntityGroupToEntitiesMap,
								resourceEntityGroup: context.resourceEntityGroup,
							}),
							onDone: [
								{
									target: "runningAddResourceEntityPrompt",
									guard: {
										type: "isResourceEntitySetToAddNew",
										params: ({ event }) => ({
											resourceEntity: event.output,
										}),
									},
									actions: assign({
										resourceEntity: ({ event }) => event.output,
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
					runningAddResourceEntityPrompt: {
						invoke: {
							id: "runningAddResourceEntityPrompt",
							src: "runAddResourceEntityPrompt",
							onDone: {
								target: "#root.downloadingTemplate",
								actions: assign({
									resourceEntity: ({ event }) => event.output,
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
					src: "downloadTemplate",
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
						target:
							"settingDownloadedTemplatePackageJsonPackagesWithoutMajorVersionConflicts",
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
			settingDownloadedTemplatePackageJsonPackagesWithoutMajorVersionConflicts:
				{
					invoke: {
						id: "settingDownloadedTemplatePackageJsonPackagesWithoutMajorVersionConflicts",
						src: "setDownloadedTemplatePackageJsonPackagesWithoutMajorVersionConflicts",
						input: ({ context }) => ({
							downloadedTemplatePackageJson:
								context.downloadedTemplatePackageJson as PackageJson,
							gasolineDirPackageJson:
								context.gasolineDirPackageJson as PackageJson,
						}),
						onDone: {
							target: "settingPackagesWithMajorVersionConflicts",
							actions: assign({
								packagesWithoutMajorVersionConflicts: ({ event }) =>
									event.output,
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

type FsGetDirFilesOptions = {
	fileRegexToMatch?: RegExp;
};

async function fsGetDirFiles(dir: string, options: FsGetDirFilesOptions = {}) {
	try {
		const result = [];
		const { fileRegexToMatch = /.*/ } = options;
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

async function gasolineDirGetPackageJson(dir: string) {
	try {
		console.log(`Getting ${dir}/package.json`);
		const packageJson = await fsPromises
			.readFile(path.join(dir, "package.json"), "utf-8")
			.then(JSON.parse);
		console.log(`Got ${dir}/package.json`);
		return packageJson;
	} catch (error) {
		console.error(error);
		throw new Error(`Unable to get ${dir}/package.json`);
	}
}

async function promptsAddResourceEntity() {
	const { resourceEntity } = await inquirer.prompt([
		{
			type: "input",
			name: "resourceEntity",
			message: "Enter resource entity",
		},
	]);
	return resourceEntity;
}

async function promptsRunAddResourceEntityGroupPrompt() {
	const { resourceEntityGroup } = await inquirer.prompt([
		{
			type: "input",
			name: "resourceEntityGroup",
			message: "Enter resource entity group",
		},
	]);
	return resourceEntityGroup;
}

async function promptsRunSelectResourceEntity(
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

async function promptsRunSelectResourceEntityGroup(
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

type ResourcesEntityGroupToEntitiesMap = Map<string, string[]>;

function resourcesSetEntityGroupToEntitiesMap(
	gasolineDirFiles: string[],
): ResourcesEntityGroupToEntitiesMap {
	const result = new Map<string, string[]>();
	for (const file of gasolineDirFiles) {
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

async function templatesDownload(options: {
	src: string;
	downloadDir: string;
	templateName: string;
}) {
	try {
		const { src, downloadDir, templateName } = options;
		console.log(`Downloading template ${src}`);
		await downloadTemplate(src, {
			dir: `${downloadDir}/${templateName}`,
			forceClean: true,
		});
		console.log(`Downloaded template ${src}`);
	} catch (error) {
		console.error(error);
		throw new Error("Unable to download template");
	}
}

async function templatesGetDownloadedPackageJson(
	downloadDir: string,
	templateName: string,
) {
	try {
		console.log("Getting downloaded template package.json");
		const packageJson = await fsPromises
			.readFile(path.join(downloadDir, templateName, "package.json"), "utf-8")
			.then(JSON.parse);
		console.log("Got downloaded template package.json");
		return packageJson;
	} catch (error) {
		console.error(error);
		throw new Error("Unable to get downloaded template package.json");
	}
}

type PackageJson = {
	dependencies?: { [key: string]: string };
	devDependencies?: { [key: string]: string };
};

function templatesSetDownloadedPackageJsonPackagesWithoutMajorVersionConflicts(
	downloadedTemplatePackageJson: PackageJson,
	gasolineDirPackageJson: PackageJson,
) {
	const result: string[] = [];
	if (
		downloadedTemplatePackageJson.dependencies &&
		Object.keys(downloadedTemplatePackageJson.dependencies).length > 0 &&
		gasolineDirPackageJson.dependencies &&
		Object.keys(gasolineDirPackageJson.dependencies).length > 0
	) {
		for (const downloadedTemplateDependency in downloadedTemplatePackageJson.dependencies) {
			if (
				gasolineDirPackageJson.dependencies[downloadedTemplateDependency] &&
				gasolineDirPackageJson.dependencies[downloadedTemplateDependency].split(
					".",
				)[0] === downloadedTemplateDependency.split(".")[0]
			) {
				result.push(downloadedTemplateDependency);
			}
		}
	}
	return result;
}
