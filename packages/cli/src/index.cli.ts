#!/usr/bin/env node
import { parseArgs } from "node:util";
import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { Miniflare } from "miniflare";
import { runAddCommand } from "./commands/cli.add.js";
import { printVerboseLogs } from "./commons/cli.log.js";

const cliOptions = {
	help: {
		type: "boolean",
		short: "h",
	},
	verbose: {
		type: "boolean",
		short: "v",
	},
	dir: {
		type: "string",
	},
	entityGroup: {
		type: "string",
	},
	resourceContainerDir: {
		type: "string",
	},
} as const;

const cliParsedArgs = parseArgs({
	allowPositionals: true,
	options: cliOptions,
});

export type CliParsedArgs = typeof cliParsedArgs;

if (cliParsedArgs.values.verbose) printVerboseLogs();

async function main() {
	try {
		const helpMessage = `Usage:
gasoline [command] -> Run command

Commands:
 add:cloudflare:worker:api:empty Add Cloudflare Worker API

Options:
 --help, -h Print help`;

		if (cliParsedArgs.positionals?.[0]) {
			const cliCommand = cliParsedArgs.positionals[0];

			const commandDoesNotExistMessage = `Command "${cliCommand}" does not exist. Run "gasoline --help" to see available commands.`;

			if (cliCommand.includes("add:")) {
				const availableAddCommands = [
					"add:cloudflare:worker:api:empty",
					"add:cloudflare:worker:api:hono",
				];

				if (availableAddCommands.includes(cliCommand)) {
					await runAddCommand(cliCommand, cliParsedArgs);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else if (cliCommand === "dev") {
				await commandsRunDev(cliParsedArgs.values);
			} else {
				console.log(commandDoesNotExistMessage);
			}
		} else {
			console.log(helpMessage);
		}
	} catch (error) {
		console.error(error);
	}
}

await main();

async function commandsRunDev(commandOptions: {
	[value: string]: boolean | string | undefined;
}) {
	//const config = await getConfig();
	//const resourceFileMap = await setResourceFileMap({
	//commandDir: commandOptions.dir,
	//configDirs: config.dirs,
	//	});
	//const resourceConfigMap = await setResourceConfigMap(resourceFileMap);
	//console.log(resourceConfigMap);
	/*
	async function getGasolineDirFiles() {
		try {
			console.log("Reading gasoline directory");
			const result: string[] = [];
			async function recursiveRead(currentPath: string) {
				const entries = await fsPromises.readdir(currentPath, {
					withFileTypes: true,
				});
				for (const entry of entries) {
					const entryPath = path.join(currentPath, entry.name);
					if (entry.isDirectory()) {
						if (entry.name !== "node_modules" && entry.name !== ".store") {
							await recursiveRead(entryPath);
						}
					} else {
						if (entry.name.split(".").length === 4) {
							result.push(entry.name);
						}
					}
				}
			}
			await recursiveRead("./gasoline");
			console.log("Read gasoline directory");
			return result;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to read gasoline directory");
		}
	}

	const gasolineDirResourceFiles = await getGasolineDirFiles();

	const readGasolineDirResourceFilePromises: Promise<string>[] = [];
	for (const file of gasolineDirResourceFiles) {
		readGasolineDirResourceFilePromises.push(
			fsPromises.readFile(`./gasoline/${file}`, "utf-8"),
		);
	}

	const readGasolineDirResourceFilePromisesResult = await Promise.all(
		readGasolineDirResourceFilePromises,
	);

	type GasolineDirResourceFileToBody = {
		[resourceFile: string]: string;
	};

	const gasolineDirResourceFileToBody: GasolineDirResourceFileToBody = {};
	for (let i = 0; i < gasolineDirResourceFiles.length; i++) {
		gasolineDirResourceFileToBody[gasolineDirResourceFiles[i]] =
			readGasolineDirResourceFilePromisesResult[i];
	}

	type GasolineDirResourceFileToExportedConfigVar = {
		[resourceFile: string]: string | undefined;
	};

	const gasolineDirResourceFileToExportedConfigVarFilteredByType: GasolineDirResourceFileToExportedConfigVar =
		{};
	for (const file in gasolineDirResourceFileToBody) {
		const mod = parseModule(gasolineDirResourceFileToBody[file]);
		if (mod.exports) {
			for (const modExport in mod.exports) {
				// Assume this is a config export for now.
				if (
					mod.exports[modExport].id &&
					mod.exports[modExport].type &&
					// filter for cloudflare-worker for now.
					// this can be an optional filter later
					// when this function is extracted.
					mod.exports[modExport].type === "cloudflare-worker"
				) {
					gasolineDirResourceFileToExportedConfigVarFilteredByType[file] =
						modExport;
					break;
				}
			}
		}
	}

	console.log(gasolineDirResourceFileToExportedConfigVarFilteredByType);

	if (
		Object.keys(gasolineDirResourceFileToExportedConfigVarFilteredByType)
			.length > 0
	) {
		for (const resourceFile in gasolineDirResourceFileToExportedConfigVarFilteredByType) {
			console.log("running esbuild");
			await esbuild.build({
				entryPoints: [path.join(`./gasoline/${resourceFile}`)],
				bundle: true,
				format: "esm",
				outfile: `./gasoline/.store/cloudflare-worker-dev-bundles/${resourceFile.replace(
					".ts",
					".js",
				)}`,
				tsconfig: "./gasoline/tsconfig.json",
			});
			console.log("ran esbuild");
		}
	}

	console.log("Starting dev server");

	const app = new Hono();

	const mf = new Miniflare({
		modules: true,
		scriptPath:
			"./gasoline/.store/cloudflare-worker-dev-bundles/core.base.api.js",
	});
	app.get("/", async (c) => {
		const fetchRes = await mf.dispatchFetch("http://localhost:8787/");
		const text = await fetchRes.text();
		return c.text(text);
	});
	serve(app, (info) => {
		console.log(`Listening on http://localhost:${info.port}`);
	});
	*/
}
