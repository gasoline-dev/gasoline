#!/usr/bin/env node

import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assign, createMachine, interpret, log } from 'xstate';
import { waitFor } from 'xstate/lib/waitFor.js';
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
			const initMachineService = interpret(setInitMachine()).start();

			const finalState = await waitFor(
				initMachineService,
				(state) => state.matches('ok') || state.matches('err'),
				{
					timeout: 3600_000,
				},
			);

			if (finalState.value === 'err') {
				throw new Error('Unable to create project');
			}

			console.log('Done!');
			process.exit(0);
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

function setInitMachine() {
	return createMachine(
		{
			predictableActionArguments: true,
			id: 'create',
			initial: 'showingDirPrompt',
			//tsTypes: {} as import('./index.create.typegen.d.ts').Typegen0,
			schema: {
				context: {} as { dir: string; workerName: string },
				services: {} as {
					copyTemplate: {
						data: void;
					};
					dirPrompt: {
						data: string;
					};
					installDependenciesPrompt: {
						data: 'yes' | 'no';
					};
					workerNamePrompt: {
						data: string;
					};
				},
			},
			context: {
				dir: '',
				workerName: '',
			},
			states: {
				showingDirPrompt: {
					invoke: {
						id: 'dirPrompt',
						src: 'dirPrompt',
						onDone: {
							target: 'showingWorkerNamePrompt',
							actions: ['setDir'],
						},
						onError: {
							target: 'err',
						},
					},
				},
				showingWorkerNamePrompt: {
					invoke: {
						id: 'workerNamePrompt',
						src: 'workerNamePrompt',
						onDone: {
							target: 'copyingTemplate',
							actions: ['setWorkerName'],
						},
						onError: {
							target: 'err',
						},
					},
				},
				copyingTemplate: {
					invoke: {
						id: 'copyTemplate',
						src: 'copyTemplate',
						onDone: {
							target: 'installDependenciesPrompt',
						},
						onError: {
							target: 'err',
						},
					},
				},
				installDependenciesPrompt: {
					initial: 'showingInstallDependenciesPrompt',
					states: {
						showingInstallDependenciesPrompt: {
							invoke: {
								id: 'installDependenciesPrompt',
								src: 'installDependenciesPrompt',
								onDone: [
									{
										target: 'installingDependencies',
										cond: 'installDependencies',
									},
									{
										target: '#create.updatingWranglerToml',
										actions: log(
											(context) =>
												`cd into ${context.dir} and run "npm install"`,
										),
									},
								],
								onError: {
									target: '#create.err',
								},
							},
						},
						installingDependencies: {
							invoke: {
								id: 'installDependencies',
								src: 'installDependencies',
								onDone: [
									{
										target: '#create.updatingWranglerToml',
									},
								],
								onError: {
									target: '#create.err',
								},
							},
						},
					},
				},
				updatingWranglerToml: {
					invoke: {
						id: 'updateWranglerToml',
						src: 'updateWranglerToml',
						onDone: [
							{
								target: 'ok',
							},
						],
						onError: {
							target: '#create.err',
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
		},
		{
			actions: {
				setDir: assign({
					// @ts-ignore
					dir: (context, event) => event.data,
				}),
				setWorkerName: assign({
					// @ts-ignore
					workerName: (context, event) => event.data,
				}),
			},
			guards: {
				installDependencies: (context, event) => {
					if (event.data === 'yes') {
						return true;
					}
					return false;
				},
			},
			services: {
				copyTemplate: async (context) => {
					console.log('Copying template');
					const src = path.resolve(
						fileURLToPath(import.meta.url),
						'../..',
						'templates/hello-world',
					);
					const dest = context.dir;
					await fsCopyDir(src, dest);
					console.log('Copied template');
				},
				dirPrompt: async () => {
					const { dirPath } = await inquirer.prompt([
						{
							name: 'dirPath',
							message: 'Dir path:',
							default: './example',
						},
					]);
					return dirPath;
				},
				installDependencies: async (context) => {
					console.log('Installing dependencies');
					const promisifiedExec = promisify(exec);
					await promisifiedExec('npm install', {
						cwd: path.resolve(context.dir),
					});
					console.log('Installed dependencies');
				},
				installDependenciesPrompt: async () => {
					const { installDependencies } = await inquirer.prompt([
						{
							name: 'installDependencies',
							message: 'Install npm dependencies?',
							type: 'list',
							choices: ['yes', 'no'],
						},
					]);
					return installDependencies;
				},
				updateWranglerToml: async (context) => {
					try {
						console.log('Updating wrangler.toml');

						const wranglerTomlPath = path.join(context.dir, './wrangler.toml');

						let contents = await fsPromises.readFile(wranglerTomlPath, {
							encoding: 'utf-8',
						});

						contents = contents.replace(/name\s*=\s*("[^"]*")/, () => {
							return `name = "${context.workerName}"`;
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
				workerNamePrompt: async () => {
					const { workerName } = await inquirer.prompt([
						{
							name: 'workerName',
							message: 'Worker name:',
							default: 'hello-world',
						},
					]);
					return workerName;
				},
			},
		},
	);
}

async function runPackageCommand() {
	const { dirPath } = await inquirer.prompt([
		{
			name: 'dirPath',
			message: 'Dir path:',
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
	const dest = dirPath;
	await fsCopyDir(src, dest);
	console.log('Copied template');

	const { installDependencies } = await inquirer.prompt([
		{
			name: 'installDependencies',
			message: 'Install npm dependencies?',
			type: 'list',
			choices: ['yes', 'no'],
		},
	]);

	if (installDependencies === 'yes') {
		console.log('Installing dependencies');
		const promisifiedExec = promisify(exec);
		await promisifiedExec('npm install', {
			cwd: path.resolve(dirPath),
		});
		console.log('Installed dependencies');
	}

	console.log('Updating package.json');

	const packageJsonPath = path.join(dirPath, './package.json');

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

async function fsCopyDir(src: string, dest: string) {
	try {
		await fsPromises.cp(src, dest, {
			recursive: true,
		});
	} catch (error) {
		console.error(error);
		throw error;
	}
}
