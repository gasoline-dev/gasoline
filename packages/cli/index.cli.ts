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
	const checkIfHiddenGasolineDirExists = fromPromise(async () => {
		try {
			console.log('Checking if ./.gasoline/templates directory exists');
			const isHiddenGasolineDirPresent = await fsIsDirPresent(
				'.gasoline/templates',
			);
			if (!isHiddenGasolineDirPresent) {
				console.log('./.gasoline/templates directory is not present');
				return false;
			}
			console.log('./.gasoline/templates directory is present');
			return true;
		} catch (error) {
			console.error(error);
			throw new Error(
				'Unable to check if ./.gasoline/templates directory exists',
			);
		}
	});

	const createHiddenGasolineDir = fromPromise(async () => {
		try {
			console.log('Creating ./.gasoline/templates directory');
			await fsPromises.mkdir('.gasoline/templates', {
				recursive: true,
			});
			console.log('Created ./.gasoline/templates directory');
		} catch (error) {
			console.error(error);
			throw new Error('Unable to create ./.gasoline/templates directory');
		}
	});

	const isHiddenGasolineDirPresent = (_, params: { isPresent: boolean }) => {
		return params.isPresent;
	};

	const downloadProvidedTemplate = fromPromise(
		async ({ input }: { input: { commandUsed: string } }) => {
			try {
				const templateName = input.commandUsed
					.replace('add:', '')
					.replace(/:/g, '-');
				const templateSource = 'github:gasoline-dev/templates/' + templateName;
				console.log('Downloading provided template ' + templateSource);
				await downloadTemplate(templateSource, {
					dir: input.commandUsed,
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
			createHiddenGasolineDir,
			downloadProvidedTemplate,
		},
		guards: {
			isHiddenGasolineDirPresent,
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
								type: 'isHiddenGasolineDirPresent',
								params: ({ event }) => ({
									isPresent: event.output,
								}),
							},
						},
						{
							target: 'creatingHiddenGasolineDir',
						},
					],
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			creatingHiddenGasolineDir: {
				invoke: {
					id: 'creatingHiddenGasolineDir',
					src: 'createHiddenGasolineDir',
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
