#!/usr/bin/env node
import { parseArgs } from 'node:util';
import inquirer from 'inquirer';
import { createActor, fromPromise, setup, waitFor } from 'xstate';
import fsPromises from 'fs/promises';
import { downloadTemplate } from 'giget';

await main();

async function main() {
	try {
		const options = {
			example: {
				type: 'string',
			},
			help: {
				type: 'boolean',
				short: 'h',
			},
		};

		const parsedArgs = parseArgs({
			allowPositionals: true,
			options,
		} as any);

		if (parsedArgs.positionals && parsedArgs.positionals[0]) {
			const command = parsedArgs.positionals[0];

			const isAddCommand = command.includes('add:') ? true : false;

			const availableAddCommands = ['add:cloudflare:worker:api:empty'];

			if (isAddCommand) {
				if (availableAddCommands.includes(command)) {
					await runAddCommandMachine(command);
				} else {
					console.log('Command ' + command + ' not found');
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
	const localTemplatesDirectory = './gasoline/.store/templates';

	const checkIfHiddenGasolineDirExists = fromPromise(async () => {
		try {
			console.log(
				'Checking if ' + localTemplatesDirectory + ' directory exists',
			);
			const isGasolineStoreTemplatesDirPresent = await fsIsDirPresent(
				localTemplatesDirectory,
			);
			if (!isGasolineStoreTemplatesDirPresent) {
				console.log(localTemplatesDirectory + ' directory is not present');
				return false;
			}
			console.log(localTemplatesDirectory + ' directory is present');
			return true;
		} catch (error) {
			console.error(error);
			throw new Error(
				'Unable to check if ' + localTemplatesDirectory + ' directory exists',
			);
		}
	});

	const createGasolineStoreTemplatesDir = fromPromise(async () => {
		try {
			console.log('Creating ' + localTemplatesDirectory + ' directory');
			await fsPromises.mkdir(localTemplatesDirectory, {
				recursive: true,
			});
			console.log('Created ' + localTemplatesDirectory + ' directory');
		} catch (error) {
			console.error(error);
			throw new Error(
				'Unable to create' + localTemplatesDirectory + ' directory',
			);
		}
	});

	const isGasolineStoreTemplatesDirPresent = (
		_,
		params: { isPresent: boolean },
	) => {
		return params.isPresent;
	};

	const downloadProvidedTemplate = fromPromise(
		async ({ input }: { input: { commandUsed: string } }) => {
			try {
				const templateName = input.commandUsed
					.replace('add:', '')
					.replace(/:/g, '-');
				const templateSource =
					'github:gasoline-dev/gasoline/templates/' + templateName;
				console.log('Downloading provided template ' + templateSource);
				await downloadTemplate(templateSource, {
					dir: localTemplatesDirectory + '/' + templateName,
					forceClean: true,
				});
				console.log('Downloaded provided template ' + templateSource);
			} catch (error) {
				console.error(error);
				throw new Error('Unable to download provided template');
			}
		},
	);

	const machine = setup({
		actors: {
			checkIfHiddenGasolineDirExists,
			createGasolineStoreTemplatesDir,
			downloadProvidedTemplate,
		},
		guards: {
			isGasolineStoreTemplatesDirPresent,
		},
	}).createMachine({
		id: 'addCommand',
		initial: 'checkingIfHiddenGasolineDirExists',
		context: {
			commandUsed,
		},
		states: {
			checkingIfHiddenGasolineDirExists: {
				invoke: {
					id: 'checkingIfHiddenGasolineDirExists',
					src: 'checkIfHiddenGasolineDirExists',
					onDone: [
						{
							target: 'downloadingTemplate',
							guard: {
								type: 'isGasolineStoreTemplatesDirPresent',
								params: ({ event }) => ({
									isPresent: event.output,
								}),
							},
						},
						{
							target: 'creatingGasolineStoreTemplatesDir',
						},
					],
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			creatingGasolineStoreTemplatesDir: {
				invoke: {
					id: 'creatingGasolineStoreTemplatesDir',
					src: 'createGasolineStoreTemplatesDir',
					onDone: {
						target: 'downloadingTemplate',
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			downloadingTemplate: {
				invoke: {
					id: 'downloadingTemplate',
					src: 'downloadProvidedTemplate',
					input: ({ context }) => ({
						commandUsed: context.commandUsed,
					}),
					onDone: {
						target: 'ok',
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			ok: {
				type: 'final',
			},
			err: {
				type: 'final',
			},
		},
	});

	const actor = createActor(machine).start();

	const snapshot = await waitFor(
		actor,
		(snapshot) => snapshot.matches('ok') || snapshot.matches('err'),
		{
			timeout: 3600_000,
		},
	);

	if (snapshot.value === 'err') {
		throw new Error('Unable to add template');
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
