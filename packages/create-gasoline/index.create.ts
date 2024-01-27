#!/usr/bin/env node

import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
	assign,
	createActor,
	createMachine,
	fromPromise,
	log,
	setup,
	waitFor,
} from 'xstate';
import fsPromises from 'fs/promises';
import inquirer from 'inquirer';
import { promisify } from 'node:util';
import { exec } from 'node:child_process';
import { parseArgs } from 'node:util';

await main();

async function main() {
	try {
		const options = {
			package: {
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

		// Initialize project (no args provided).
		if (
			parsedArgs.positionals.length === 0 &&
			Object.keys(parsedArgs.values).length === 0
		) {
			await runInitMachine();
		}

		// Initialize single repo package.
		if (parsedArgs.positionals[0] === 'package') {
			await runPackageCommand();
			process.exit(0);
		}

		// Log help.
		if (parsedArgs.values.help) {
			logHelp();
			process.exit(0);
		}
	} catch (error) {
		console.error(error);
	}
}

async function runInitMachine() {
	const runSetDirPrompt = fromPromise(async () => {
		const { directoryPath } = await inquirer.prompt([
			{
				name: 'directoryPath',
				message: 'Directory path:',
				default: './example',
			},
		]);
		return directoryPath;
	});

	const runSetWorkerNamePrompt = fromPromise(async () => {
		const { workerName } = await inquirer.prompt([
			{
				name: 'workerName',
				message: 'Worker name:',
				default: 'hello-world',
			},
		]);
		return workerName;
	});

	const copyTemplate = fromPromise(
		async ({
			input,
		}: {
			input: {
				directory: string;
			};
		}) => {
			console.log('Copying template');
			const src = path.resolve(
				fileURLToPath(import.meta.url),
				'../..',
				'templates/hello-world',
			);
			const dest = input.directory;
			await fsCopyDirectory(src, dest);
			console.log('Copied template');
		},
	);

	const installDependencies = fromPromise(
		async ({
			input,
		}: {
			input: {
				directory: string;
			};
		}) => {
			console.log('Installing dependencies');
			const promisifiedExec = promisify(exec);
			await promisifiedExec('npm install', {
				cwd: path.resolve(input.directory),
			});
			console.log('Installed dependencies');
		},
	);

	const updateWranglerToml = fromPromise(
		async ({
			input,
		}: {
			input: {
				directory: string;
				workerName: string;
			};
		}) => {
			try {
				console.log('Updating wrangler.toml');

				const wranglerTomlPath = path.join(input.directory, './wrangler.toml');

				let contents = await fsPromises.readFile(wranglerTomlPath, {
					encoding: 'utf-8',
				});

				contents = contents.replace(/name\s*=\s*("[^"]*")/, () => {
					return `name = "${input.workerName}"`;
				});

				contents = contents.replace(
					/compatibility_date\s*=\s*("[^"]*")/,
					() => {
						const newDate = new Date().toISOString().split('T')[0];
						return `compatibility_date = "${newDate}"`;
					},
				);

				await fsPromises.writeFile(wranglerTomlPath, contents, 'utf-8');

				console.log('Updated wrangler.toml');
			} catch (error) {
				console.error(error);
				console.error('Error: Unable to update wrangler.toml');
			}
		},
	);

	const machine = setup({
		actors: {
			runSetDirPrompt,
			runSetWorkerNamePrompt,
			copyTemplate,
			installDependencies,
			updateWranglerToml,
		},
		types: {} as {
			context: {
				directory: string;
				workerName: string;
			};
		},
	}).createMachine({
		id: 'create',
		initial: 'runningSetDirPrompt',
		context: {
			directory: '',
			workerName: '',
		},
		states: {
			runningSetDirPrompt: {
				invoke: {
					id: 'runningSetDirPrompt',
					src: 'runSetDirPrompt',
					onDone: {
						target: 'runningSetWorkerNamePrompt',
						actions: assign({
							directory: ({ event }) => event.output,
						}),
					},
					onError: {
						target: 'err',
					},
				},
			},
			runningSetWorkerNamePrompt: {
				invoke: {
					id: 'runningSetWorkerNamePrompt',
					src: 'runSetWorkerNamePrompt',
					onDone: {
						target: 'copyingTemplate',
						actions: assign({
							workerName: ({ event }) => event.output,
						}),
					},
					onError: {
						target: 'err',
					},
				},
			},
			copyingTemplate: {
				invoke: {
					id: 'copyingTemplate',
					src: 'copyTemplate',
					input: ({ context }) => ({
						directory: context.directory,
					}),
					onDone: {
						target: 'installingDependencies',
					},
					onError: {
						target: 'err',
					},
				},
			},
			installingDependencies: {
				invoke: {
					id: 'installingDependencies',
					src: 'installDependencies',
					input: ({ context }) => ({
						directory: context.directory,
					}),
					onDone: {
						target: 'updatingWranglerToml',
					},
					onError: {
						target: 'err',
					},
				},
			},
			updatingWranglerToml: {
				invoke: {
					id: 'updatingWranglerToml',
					src: 'updateWranglerToml',
					input: ({ context }) => ({
						directory: context.directory,
						workerName: context.workerName,
					}),
					onDone: {
						target: 'ok',
					},
					onError: {
						target: 'err',
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
		throw new Error('Unable to create project');
	}

	console.log('Done!');

	process.exit(0);
}

async function runPackageCommand() {
	const { directoryPath } = await inquirer.prompt([
		{
			name: 'directoryPath',
			message: 'Directory path:',
			default: './example',
		},
	]);

	const { packageName } = await inquirer.prompt([
		{
			name: 'packageName',
			message: 'Package name:',
			default: 'example-name',
		},
	]);

	console.log('Copying template');
	const src = path.resolve(
		fileURLToPath(import.meta.url),
		'../..',
		'templates/package',
	);
	const destination = directoryPath;
	await fsCopyDirectory(src, destination);
	console.log('Copied template');

	console.log('Installing dependencies');
	const promisifiedExec = promisify(exec);
	await promisifiedExec('npm install', {
		cwd: path.resolve(directoryPath),
	});
	console.log('Installed dependencies');

	console.log('Updating package.json');

	const packageJsonPath = path.join(directoryPath, './package.json');

	let contents = await fsPromises.readFile(packageJsonPath, {
		encoding: 'utf-8',
	});

	const parsedContents = JSON.parse(contents);

	parsedContents.name = packageName;

	await fsPromises.writeFile(
		packageJsonPath,
		JSON.stringify(parsedContents, null, 2),
		'utf-8',
	);

	console.log('Updated package.json');

	console.log('Done!');
}

function logHelp() {
	console.log(`Usage:
create-gasoline -> Initalize project

OR

create-gasoline [command] -> Run command

Commands:
 package Initalize a single repo package for publishing to NPM

Options:
 --help, -h Print help`);
}

async function fsCopyDirectory(src: string, dest: string) {
	try {
		await fsPromises.cp(src, dest, {
			recursive: true,
		});
	} catch (error) {
		console.error(error);
		throw error;
	}
}
