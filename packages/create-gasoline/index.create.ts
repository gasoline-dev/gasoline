import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assign, createMachine, interpret, log } from 'xstate';
import { waitFor } from 'xstate/lib/waitFor.js';
import fsPromises from 'fs/promises';
import inquirer from 'inquirer';
import { promisify } from 'node:util';
import { exec } from 'node:child_process';

await main();

async function main() {
	try {
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
	} catch (error) {
		console.error(error);
	}
}

function setInitMachine() {
	return createMachine(
		{
			predictableActionArguments: true,
			id: 'create',
			initial: 'showingInitDirPrompt',
			tsTypes: {} as import('./index.create.typegen.d.ts').Typegen0,
			schema: {
				context: {} as { initDir: string },
				services: {} as {
					copyTemplate: {
						data: void;
					};
					initDirPrompt: {
						data: string;
					};
					installDependenciesPrompt: {
						data: 'yes' | 'no';
					};
				},
			},
			context: {
				initDir: '',
			},
			states: {
				showingInitDirPrompt: {
					invoke: {
						id: 'initDirPrompt',
						src: 'initDirPrompt',
						onDone: {
							target: 'copyingTemplate',
							actions: ['setInitDir'],
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
										target: '#create.ok',
										actions: log(
											(context) =>
												`cd into ${context.initDir} and run "npm install"`,
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
										target: '#create.ok',
									},
								],
								onError: {
									target: '#create.err',
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
		},
		{
			actions: {
				setInitDir: assign({
					initDir: (context, event) => event.data,
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
					const dest = context.initDir;
					await fsCopyDir(src, dest);
					console.log('Copied template');
				},
				initDirPrompt: async () => {
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
						cwd: path.resolve(context.initDir),
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
			},
		},
	);
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
