#!/usr/bin/env node
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assign, createActor, fromPromise, setup, waitFor } from 'xstate';
import fsPromises from 'fs/promises';
import inquirer from 'inquirer';
import { promisify } from 'node:util';
import { exec } from 'node:child_process';
import { parseArgs } from 'node:util';
import { downloadTemplate } from 'giget';

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
		try {
			const { directoryPath } = await inquirer.prompt([
				{
					name: 'directoryPath',
					message: 'Directory path:',
					default: './example',
				},
			]);
			return directoryPath;
		} catch (error) {
			console.error(error);
			throw new Error('Unable to set directory path');
		}
	});

	const checkIfDirIsPresent = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			console.log('Checking if directory is present');
			try {
				const isDirPresent = await fsIsDirPresent(input.directory);
				if (!isDirPresent) {
					console.log('Directory is not present');
					return false;
				}
				console.log('Directory is present');
				return true;
			} catch (error) {
				console.error(error);
				throw new Error('Unable to check if directory is present');
			}
		},
	);

	const isDirPresent = (_, params: { isPresent: boolean }) => {
		return params.isPresent;
	};

	const getDirContents = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			try {
				console.log('Getting directory contents');
				const contents = await fsPromises.readdir(input.directory);
				console.log('Got directory contents');
				return contents;
			} catch (error) {
				console.error(error);
				throw new Error('Unable to get directory contents');
			}
		},
	);

	const isDirEmpty = (_, params: { contents: string[] }) => {
		return params.contents.length === 0;
	};

	const runEmptyDirContentsConfirmPrompt = fromPromise(async () => {
		try {
			const { confirm } = await inquirer.prompt([
				{
					type: 'confirm',
					name: 'confirm',
					message: 'Directory is not empty. Empty it?',
					default: false,
				},
			]);
			return confirm;
		} catch (error) {
			console.error(error);
			throw new Error('Unable to confirm if directory should be emptied');
		}
	});

	const isConfirmedToEmptyDir = (_, params: { confirm: boolean }) => {
		return params.confirm;
	};

	const emptyDirContents = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			try {
				console.log('Emptying directory contents');
				const contents = await fsPromises.readdir(input.directory);
				await Promise.all(
					contents.map((file) => {
						return fsPromises.rm(path.join(input.directory, file), {
							recursive: true,
						});
					}),
				);
				console.log('Emptied directory contents');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to empty directory contents');
			}
		},
	);

	function logEmptyDirIsRequiredMessage() {
		console.log(
			'create-gasoline is for new projects and requires an empty directory',
		);
	}

	const runSetWorkerNamePrompt = fromPromise(async () => {
		try {
			const { workerName } = await inquirer.prompt([
				{
					name: 'workerName',
					message: 'Worker name:',
					default: 'hello-world',
				},
			]);
			return workerName;
		} catch (error) {
			console.error(error);
			throw new Error('Unable to set worker name');
		}
	});

	const getTemplate = fromPromise(
		async ({
			input,
		}: {
			input: {
				directory: string;
			};
		}) => {
			try {
				console.log('Getting template');
				await downloadTemplate(
					'github:gasoline-dev/gasoline/templates/hello-world',
					{
						dir: input.directory,
					},
				);
				console.log('Got template');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to get template');
			}
		},
	);

	const runSetPackageManagerPrompt = fromPromise(async () => {
		try {
			const { packageManager } = await inquirer.prompt([
				{
					type: 'list',
					name: 'packageManager',
					message: 'Package manager:',
					choices: ['npm', 'pnpm'],
					default: 'npm',
				},
			]);
			return packageManager;
		} catch (error) {
			console.error(error);
			throw new Error('Unable to set package manager');
		}
	});

	const installDependencies = fromPromise(
		async ({
			input,
		}: {
			input: {
				directory: string;
				packageManager: 'npm' | 'pnpm';
			};
		}) => {
			try {
				console.log('Installing dependencies');
				const promisifiedExec = promisify(exec);
				switch (input.packageManager) {
					case 'npm':
						await promisifiedExec('npm install', {
							cwd: path.resolve(input.directory),
						});
						break;
					case 'pnpm':
						await promisifiedExec('pnpm install', {
							cwd: path.resolve(input.directory),
						});
						break;
					default:
						const never: never = input.packageManager;
						throw new Error('Error: Unknown package manager ->' + never);
				}
				console.log('Installed dependencies');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to install dependencies');
			}
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
				throw new Error('Unable to update wrangler.toml');
			}
		},
	);

	const machine = setup({
		actions: {
			logEmptyDirIsRequiredMessage,
		},
		actors: {
			runSetDirPrompt,
			checkIfDirIsPresent,
			getDirContents,
			runEmptyDirContentsConfirmPrompt,
			emptyDirContents,
			runSetWorkerNamePrompt,
			getTemplate,
			installDependencies,
			runSetPackageManagerPrompt,
			updateWranglerToml,
		},
		guards: {
			isDirPresent,
			isDirEmpty,
			isConfirmedToEmptyDir,
		},
		types: {} as {
			actions: {
				type: 'logEmptyDirIsRequiredMessage';
			};
			context: {
				directory: string;
				packageManager: 'npm' | 'pnpm';
				workerName: string;
			};
			guards:
				| { type: 'isDirPresent' }
				| {
						type: 'isDirEmpty';
				  }
				| {
						type: 'isConfirmedToEmptyDir';
				  };
		},
	}).createMachine({
		id: 'create',
		initial: 'runningSetDirPrompt',
		context: {
			directory: '',
			packageManager: 'npm',
			workerName: '',
		},
		states: {
			runningSetDirPrompt: {
				invoke: {
					id: 'runningSetDirPrompt',
					src: 'runSetDirPrompt',
					onDone: {
						target: 'checkingIfDirIsPresent',
						actions: assign({
							directory: ({ event }) => event.output,
						}),
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			checkingIfDirIsPresent: {
				invoke: {
					id: 'checkingIfDirIsPresent',
					src: 'checkIfDirIsPresent',
					input: ({ context }) => ({
						directory: context.directory,
					}),
					onDone: [
						{
							target: '#processingDirContents',
							guard: {
								type: 'isDirPresent',
								params: ({ event }) => ({
									isPresent: event.output,
								}),
							},
						},
						{
							target: 'runningSetWorkerNamePrompt',
						},
					],
					onError: {
						target: '#create.err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			processingDirContents: {
				id: 'processingDirContents',
				initial: 'gettingDirContents',
				states: {
					gettingDirContents: {
						invoke: {
							id: 'gettingDirContents',
							src: 'getDirContents',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: [
								{
									target: '#create.runningSetWorkerNamePrompt',
									guard: {
										type: 'isDirEmpty',
										params: ({ context, event }) => ({
											contents: event.output,
										}),
									},
								},
								{
									target: 'runningEmptyDirContentsConfirmPrompt',
								},
							],
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					runningEmptyDirContentsConfirmPrompt: {
						invoke: {
							id: 'runningEmptyDirContentsConfirmPrompt',
							src: 'runEmptyDirContentsConfirmPrompt',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: [
								{
									target: 'emptyingDirContents',
									guard: {
										type: 'isConfirmedToEmptyDir',
										params: ({ event }) => ({
											confirm: event.output,
										}),
									},
								},
								{
									target: 'emptyDirRequired',
								},
							],
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					emptyingDirContents: {
						invoke: {
							id: 'emptyingDirContents',
							src: 'emptyDirContents',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: {
								target: '#create.runningSetWorkerNamePrompt',
							},
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					emptyDirRequired: {
						entry: {
							type: 'logEmptyDirIsRequiredMessage',
						},
						always: {
							target: '#create.ok',
						},
					},
				},
			},
			runningSetWorkerNamePrompt: {
				invoke: {
					id: 'runningSetWorkerNamePrompt',
					src: 'runSetWorkerNamePrompt',
					onDone: {
						target: 'gettingTemplate',
						actions: assign({
							workerName: ({ event }) => event.output,
						}),
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			gettingTemplate: {
				invoke: {
					id: 'gettingTemplate',
					src: 'getTemplate',
					input: ({ context }) => ({
						directory: context.directory,
					}),
					onDone: {
						target: 'runningSetPackageManagerPrompt',
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			runningSetPackageManagerPrompt: {
				invoke: {
					id: 'runningSetPackageManagerPrompt',
					src: 'runSetPackageManagerPrompt',
					onDone: {
						target: 'installingDependencies',
						actions: assign({
							packageManager: ({ event }) => event.output,
						}),
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			installingDependencies: {
				invoke: {
					id: 'installingDependencies',
					src: 'installDependencies',
					input: ({ context }) => ({
						directory: context.directory,
						packageManager: context.packageManager,
					}),
					onDone: {
						target: 'updatingWranglerToml',
					},
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
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
		throw new Error('Unable to create project');
	}

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
	await fsCopyDir(src, destination);
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

async function fsIsDirPresent(directory: string) {
	try {
		await fsPromises.access(directory);
		return true;
	} catch (error) {
		return false;
	}
}
