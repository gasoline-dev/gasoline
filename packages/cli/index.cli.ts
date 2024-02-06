#!/usr/bin/env node
import { parseArgs } from "node:util";
import inquirer from "inquirer";
import { assign, createActor, fromPromise, setup, waitFor } from "xstate";
import fsPromises from "fs/promises";
import { downloadTemplate } from "giget";
import path from "node:path";

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
		};

		const parsedArgs = parseArgs({
			allowPositionals: true,
			options,
		} as any);

		if (parsedArgs.positionals && parsedArgs.positionals[0]) {
			const command = parsedArgs.positionals[0];

			const isAddCommand = command.includes("add:") ? true : false;

			const availableAddCommands = [
				"add:cloudflare:worker:api:empty",
				"add:cloudflare:worker:api:hono",
			];

			if (isAddCommand) {
				if (availableAddCommands.includes(command)) {
					await runAddCommandMachine(command);
				} else {
					console.log("Command " + command + " not found");
				}
			}
		} else {
			logHelp();
		}
	} catch (error) {
		console.error(error);
	}
}

async function runAddCommandMachine(commandUsed: string) {
	const gasolineDirectory = "./gasoline";
	const localTemplatesDirectory = "./gasoline/.store/templates";
	const templateName = commandUsed.replace("add:", "").replace(/:/g, "-");
	const templateSource =
		"github:gasoline-dev/gasoline/templates/" + templateName;

	const checkIfGasolineStoreTemplatesDirExists = fromPromise(async () => {
		try {
			console.log(
				"Checking if " + localTemplatesDirectory + " directory exists",
			);
			const isGasolineStoreTemplatesDirPresent = await fsIsDirPresent(
				localTemplatesDirectory,
			);
			if (!isGasolineStoreTemplatesDirPresent) {
				console.log(localTemplatesDirectory + " directory is not present");
				return false;
			}
			console.log(localTemplatesDirectory + " directory is present");
			return true;
		} catch (error) {
			console.error(error);
			throw new Error(
				"Unable to check if " + localTemplatesDirectory + " directory exists",
			);
		}
	});

	const createGasolineStoreTemplatesDir = fromPromise(async () => {
		try {
			console.log("Creating " + localTemplatesDirectory + " directory");
			await fsPromises.mkdir(localTemplatesDirectory, {
				recursive: true,
			});
			console.log("Created " + localTemplatesDirectory + " directory");
		} catch (error) {
			console.error(error);
			throw new Error(
				"Unable to create" + localTemplatesDirectory + " directory",
			);
		}
	});

	const isGasolineStoreTemplatesDirPresent = (
		_,
		params: { isPresent: boolean },
	) => {
		return params.isPresent;
	};

	const downloadProvidedTemplate = fromPromise(async () => {
		try {
			console.log("Downloading provided template " + templateSource);
			await downloadTemplate(templateSource, {
				dir: localTemplatesDirectory + "/" + templateName,
				forceClean: true,
			});
			console.log("Downloaded provided template " + templateSource);
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

	const setPackagesWithMajorVersionConflicts = fromPromise(
		async ({
			input,
		}: {
			input: {
				downloadedTemplatePackageJson: any;
				gasolineDirPackageJson: any;
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
				for (const downloadedTemplateDep in input.downloadedTemplatePackageJson
					.dependencies) {
					if (
						input.gasolineDirPackageJson.dependencies[downloadedTemplateDep] &&
						input.gasolineDirPackageJson.dependencies[
							downloadedTemplateDep
						].split(".")[0] !== downloadedTemplateDep.split(".")[0]
					) {
						result.push(downloadedTemplateDep);
					}
				}
			}
			return result;
		},
	);

	const isThereAMajorVersionPackageConflict = (
		_,
		params: { packagesWithMajorVersionConflicts: string[] },
	) => {
		return params.packagesWithMajorVersionConflicts.length > 0;
	};

	const runResolveMajorVersionPackageConflictPrompt = fromPromise(async () => {
		const answers = await inquirer.prompt([
			{
				type: "list",
				name: "resolveMajorVersionPackageConflict",
				message:
					"There are major version package conflicts. What would you like to do?",
				choices: ["Resolve", "Use Alias", "Cancel"],
				default: "Cancel",
			},
		]);

		switch (answers.resolveMajorVersionPackageConflict) {
			case "Resolve":
				console.log("Resolving major version package conflicts");
				break;
			case "Use Alias":
				console.log("Using alias to resolve major version package conflicts");
				break;
			default:
				console.log("Not resolving major version package conflicts");
				break;
		}

		return answers.resolveMajorVersionPackageConflict;
	});

	const machine = setup({
		actors: {
			checkIfGasolineStoreTemplatesDirExists,
			createGasolineStoreTemplatesDir,
			downloadProvidedTemplate,
			getDownloadedTemplatePackageJson,
			getGasolineDirPackageJson,
			setPackagesWithMajorVersionConflicts,
			runResolveMajorVersionPackageConflictPrompt,
		},
		guards: {
			isGasolineStoreTemplatesDirPresent,
			isThereAMajorVersionPackageConflict,
		},
	}).createMachine({
		id: "addCommand",
		initial: "checkingIfGasolineStoreTemplatesDirExists",
		context: {
			commandUsed,
			downloadedTemplatePackageJson: undefined,
			gasolineDirPackageJson: undefined,
			packagesWithMajorVersionConflicts: [],
		},
		states: {
			checkingIfGasolineStoreTemplatesDirExists: {
				invoke: {
					id: "checkingIfGasolineStoreTemplatesDirExists",
					src: "checkIfGasolineStoreTemplatesDirExists",
					onDone: [
						{
							target: "downloadingTemplate",
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
			downloadingTemplate: {
				invoke: {
					id: "downloadingTemplate",
					src: "downloadProvidedTemplate",
					onDone: {
						target: "processingPackageJsons",
					},
					onError: {
						target: "err",
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			processingPackageJsons: {
				type: "parallel",
				states: {
					processingDownloadedTemplatePackageJson: {
						initial: "gettingPackageJson",
						states: {
							gettingPackageJson: {
								invoke: {
									id: "gettingDownloadedTemplatePackageJson",
									src: "getDownloadedTemplatePackageJson",
									onDone: {
										target: "gotPackageJson",
										actions: assign({
											downloadedTemplatePackageJson: ({ event }) =>
												event.output,
										}),
									},
									onError: {
										target: "#addCommand.err",
										actions: ({ context, event }) => console.error(event),
									},
								},
							},
							gotPackageJson: {
								type: "final",
							},
						},
					},
					processingGasolineDirPackageJson: {
						initial: "gettingPackageJson",
						states: {
							gettingPackageJson: {
								invoke: {
									id: "gettingGasolineDirPackageJson",
									src: "getGasolineDirPackageJson",
									onDone: {
										target: "gotPackageJson",
										actions: assign({
											gasolineDirPackageJson: ({ event }) => event.output,
										}),
									},
									onError: {
										target: "#addCommand.err",
										actions: ({ context, event }) => console.error(event),
									},
								},
							},
							gotPackageJson: {
								type: "final",
							},
						},
					},
				},
				onDone: {
					target: "processingPackageJsonVersions",
				},
			},
			processingPackageJsonVersions: {
				initial: "settingPackagesWithMajorVersionConflicts",
				states: {
					settingPackagesWithMajorVersionConflicts: {
						invoke: {
							id: "settingPackagesWithMajorVersionConflicts",
							src: "setPackagesWithMajorVersionConflicts",
							input: ({ context }) => ({
								downloadedTemplatePackageJson:
									context.downloadedTemplatePackageJson,
								gasolineDirPackageJson: context.gasolineDirPackageJson,
							}),
							onDone: [
								{
									target: "runResolveMajorVersionPackageConflictPrompt",
									guard: {
										type: "isThereAMajorVersionPackageConflict",
										params: ({ event }) => ({
											packagesWithMajorVersionConflicts: event.output,
										}),
									},
								},
								{
									target: "#addCommand.ok",
								},
							],
							onError: {
								target: "#addCommand.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runResolveMajorVersionPackageConflictPrompt: {
						invoke: {
							id: "runResolveMajorVersionPackageConflictPrompt",
							src: "runResolveMajorVersionPackageConflictPrompt",
							onDone: {
								target: "#addCommand.ok",
							},
							onError: {
								target: "#addCommand.err",
								actions: ({ context, event }) => console.error(event),
							},
						},
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
