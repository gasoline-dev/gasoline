#!/usr/bin/env node
import { parseArgs } from 'node:util';
import inquirer from 'inquirer';

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

		if (parsedArgs.values.help) {
			logHelp();
			process.exit(0);
		}
	} catch (error) {
		console.error(error);
	}
}

function logHelp() {
	console.log(`Usage:
gasoline [command] -> Run command
gas [command] -> Run command

Commands:
 Example command description

Options:
 --help, -h Print help`);
}
