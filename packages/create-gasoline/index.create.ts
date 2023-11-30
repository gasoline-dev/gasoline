import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assign, createMachine, interpret } from 'xstate';
import { waitFor } from 'xstate/lib/waitFor.js';
import fsPromises from 'fs/promises';
import inquirer from 'inquirer';

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
		},
		{
			actions: {
				setInitDir: assign({
					initDir: (context, event) => event.data,
				}),
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
