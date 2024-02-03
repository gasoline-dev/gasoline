#!/usr/bin/env node
import path from 'node:path';
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
			help: {
				type: 'boolean',
				short: 'h',
			},
		};

		const parsedArgs = parseArgs({
			allowPositionals: true,
			options,
		} as any);

		if (
			parsedArgs.positionals.length === 0 &&
			Object.keys(parsedArgs.values).length === 0
		) {
			await runInitMachine();
		} else {
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

	const isPackageManagerPnpm = (
		_,
		params: { packageManager: 'npm' | 'pnpm' },
	) => {
		return params.packageManager === 'pnpm';
	};

	const downloadMonorepoPnpmTemplate = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			try {
				console.log('Downloading monorepo pnpm template');
				await downloadTemplate(
					'github:gasoline-dev/gasoline/templates/create-gasoline-monorepo-pnpm',
					{
						dir: input.directory,
					},
				);
				console.log('Downloaded monorepo pnpm template');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to download monorepo pnpm template');
			}
		},
	);

	const installPnpmDependencies = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			try {
				console.log('Installing pnpm dependencies');
				const promisifiedExec = promisify(exec);
				await promisifiedExec('pnpm install', {
					cwd: path.resolve(input.directory),
				});
				console.log('Installed pnpm dependencies');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to install pnpm dependencies');
			}
		},
	);

	const downloadMonorepoNpmTemplate = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			try {
				console.log('Downloading monorepo npm template');
				await downloadTemplate(
					'github:gasoline-dev/gasoline/templates/create-gasoline-monorepo-npm',
					{
						dir: input.directory,
					},
				);
				console.log('Downloaded monorepo npm template');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to download monorepo npm template');
			}
		},
	);

	const installNpmDependencies = fromPromise(
		async ({ input }: { input: { directory: string } }) => {
			try {
				console.log('Installing npm dependencies');
				const promisifiedExec = promisify(exec);
				await promisifiedExec('npm install', {
					cwd: path.resolve(input.directory),
				});
				console.log('Installed npm dependencies');
			} catch (error) {
				console.error(error);
				throw new Error('Unable to install npm dependencies');
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
			runSetPackageManagerPrompt,
			downloadMonorepoPnpmTemplate,
			installPnpmDependencies,
			downloadMonorepoNpmTemplate,
			installNpmDependencies,
		},
		guards: {
			isDirPresent,
			isDirEmpty,
			isConfirmedToEmptyDir,
			isPackageManagerPnpm,
		},
		types: {} as {
			actions: {
				type: 'logEmptyDirIsRequiredMessage';
			};
			context: {
				directory: string;
				packageManager: 'npm' | 'pnpm';
			};
			guards:
				| { type: 'isDirPresent' }
				| {
						type: 'isDirEmpty';
				  }
				| {
						type: 'isConfirmedToEmptyDir';
				  }
				| {
						type: 'isPackageManagerPnpm';
				  };
		},
	}).createMachine({
		id: 'create',
		initial: 'runningSetDirPrompt',
		context: {
			directory: '',
			packageManager: 'npm',
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
							target: 'runningSetPackageManagerPrompt',
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
									target: '#create.runningSetPackageManagerPrompt',
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
								target: '#create.runningSetPackageManagerPrompt',
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
			runningSetPackageManagerPrompt: {
				invoke: {
					id: 'runningSetPackageManagerPrompt',
					src: 'runSetPackageManagerPrompt',
					onDone: [
						{
							target: '#processingMonorepoPnpmTemplate',
							guard: {
								type: 'isPackageManagerPnpm',
								params: ({ event }) => ({
									packageManager: event.output,
								}),
							},
						},
						{
							target: '#processingMonorepoNpmTemplate',
						},
					],
					onError: {
						target: 'err',
						actions: ({ context, event }) => console.error(event),
					},
				},
			},
			processingMonorepoPnpmTemplate: {
				id: 'processingMonorepoPnpmTemplate',
				initial: 'downloadingMonorepoPnpmTemplate',
				states: {
					downloadingMonorepoPnpmTemplate: {
						invoke: {
							id: 'downloadingMonorepoPnpmTemplate',
							src: 'downloadMonorepoPnpmTemplate',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: {
								target: 'installingPnpmDependencies',
							},
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					installingPnpmDependencies: {
						invoke: {
							id: 'installingPnpmDependencies',
							src: 'installPnpmDependencies',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: {
								target: '#create.ok',
							},
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
				},
			},
			processingMonorepoNpmTemplate: {
				id: 'processingMonorepoNpmTemplate',
				initial: 'downloadingMonorepoNpmTemplate',
				states: {
					downloadingMonorepoNpmTemplate: {
						invoke: {
							id: 'downloadingMonorepoNpmTemplate',
							src: 'downloadMonorepoNpmTemplate',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: {
								target: 'installingNpmDependencies',
							},
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
					},
					installingNpmDependencies: {
						invoke: {
							id: 'installingNpmDependencies',
							src: 'installNpmDependencies',
							input: ({ context }) => ({
								directory: context.directory,
							}),
							onDone: {
								target: '#create.ok',
							},
							onError: {
								target: '#create.err',
								actions: ({ context, event }) => console.error(event),
							},
						},
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

function logHelp() {
	console.log(`Usage:
create-gasoline -> Initalize project

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
