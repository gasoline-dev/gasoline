import path from "path";
import http from "http";
import { getConfig } from "../commons/cli.config.js";
import { log, spin } from "../commons/cli.log.js";
import {
	ResourceIndexFiles,
	getResourceIndexFiles,
	setResourceContainerDirs,
} from "../commons/cli.resources.js";
import { CliParsedArgs } from "../index.cli.js";
import express, { Express } from "express";
import { Miniflare } from "miniflare";
import { Readable } from "stream";
import fsPromises from "fs/promises";
import chokidar from "chokidar";
import { getPackageManager } from "../commons/cli.packages.js";
import { spawn } from "child_process";
import { loadFile, writeFile } from "magicast";

export async function runDevCommand(cliParsedArgs: CliParsedArgs) {
	//spin.start("Getting resources");
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceIndexFiles = await getResourceIndexFiles(
			resourceContainerDirs,
		);

		let startingPort = 8787;
		for (const resourceIndexFile of resourceIndexFiles) {
			const splitResourceIndexFile = path
				.basename(resourceIndexFile)
				.split(".");
			const resourceEntityGroup = splitResourceIndexFile[0].replace("_", "");
			const resourceEntity = splitResourceIndexFile[1];
			const resourceDescriptor = splitResourceIndexFile[2];
			if (resourceDescriptor === "api") {
				const availablePort = await findAvailablePort(startingPort);
				const name = `${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`;
				const main = `src/${path.basename(resourceIndexFile)}`;

				const wranglerBody = `name = "${name}"
main = "${main}"

[dev]
port = ${availablePort}
`;

				const resourceDir = path.dirname(path.dirname(resourceIndexFile));

				await fsPromises.writeFile(
					path.join(resourceDir, ".wrangler.toml"),
					wranglerBody,
				);

				startingPort = availablePort + 1;
			}
		}

		const watcher = chokidar.watch(
			[
				"gasoline/*/.wrangler/tmp/dev-**/*.*.*.index.js",
				"gasoline/*/dist/*.*.*.index.js",
			],
			{
				ignoreInitial: true,
				persistent: true,
			},
		);

		watcher
			.on("add", async (watchedPath) => {
				console.log(`File ${watchedPath} has been added`);
			})
			.on("change", async (watchedPath) => {
				console.log(`File ${watchedPath} has been changed`);

				// Normalize the path to ensure it's in a standard format
				const normalizedPath = path.normalize(watchedPath);

				// Split the path into parts using path.sep as the separator to account for different OS path separators
				const parts = normalizedPath.split(path.sep);

				// The first directory name; parts[0] would be the root on absolute paths, so parts[1] is typically the first directory
				const resourceContainerDir = parts[0];

				console.log("resourceContainerDir");
				console.log(resourceContainerDir);

				const splitWatchedPath = path.basename(watchedPath).split(".");

				const resourceEntityGroup = splitWatchedPath[0].replace("_", "");
				const resourceEntity = splitWatchedPath[1];
				const resourceDescriptor = splitWatchedPath[2];

				const resourceIndexTsFile = path.join(
					resourceContainerDir,
					`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
					"src",
					path.basename(watchedPath).replace(".js", ".ts"),
				);

				console.log("resourceIndexTsFile");
				console.log(resourceIndexTsFile);

				// user -> updates _core.base.kv.index.ts ->
				// core-base-api is dependent on core-base-kv, so ...
				// wrangler updates core-base-api/.wrangler.toml because
				// core-base-api/_core.base.api.index.ts changes.
				// gasoline dev is watching core-base-api/.wrangler/tmp/dev-abc/_core.base.api.index.js
				// and then updates core-base-api/.wrangler.toml.

				const mod = await loadFile(resourceIndexTsFile);

				for (const modExport in mod.exports) {
					if (mod.exports[modExport].type && mod.exports[modExport].id) {
						//
					}
				}

				/*
				if (path === "gasoline/core-base-kv/dist/_core.base.kv.index.js") {
					console.log("hello");
					const mod = await loadFile(
						"gasoline/core-base-kv/src/_core.base.kv.index.ts",
					);
					mod.exports.coreBaseKvConfig.namespace = "CORE_BASE_KV_TEST";
					await writeFile(
						mod,
						"gasoline/core-base-kv/src/_core.base.kv.index.ts",
					);
				}
				*/
			})
			.on("unlink", (path) => console.log(`File ${path} has been removed`));

		//const packageManager = await getPackageManager();

		/*
		const process = spawn(packageManager, ["run", "dev:all"], {
			shell: true,
			stdio: "inherit",
		});

		process.on("exit", (code) => {
			console.log(`Child process exited with code ${code}`);
		});
		*/

		return;

		/*
		const resourceDistFiles = setResourceIndexDistFiles(resourceIndexFiles);

		const resourceDistFileExports =
			await getResourceDistFileExports(resourceDistFiles);

		const resourceDistFileToConfigMap = setResourceDistFileToConfigMap(
			resourceDistFiles,
			resourceDistFileExports,
		);

		const resourceDistFileToDevIdMap = setResourceDistFileToDevIdMap();

		const devIdToMiniflareInstanceMap = setDevIdToMiniflareMap();

		const devIdToExpressInstanceMap = setDevIdToExpressInstanceMap();
		*/

		//spin.stop();

		/*
		let devId = 0;
		let startingPort = 8787;
		for (const [
			resourceDistFile,
			resourceDistConfig,
		] of resourceDistFileToConfigMap) {
			const availablePort = await findAvailablePort(startingPort);

			const miniflareInstance = setMiniflareInstance(
				resourceDistFile,
				availablePort,
			);

			devIdToMiniflareInstanceMap.set(devId, {
				instance: miniflareInstance,
				port: availablePort,
			});

			log.info(`Miniflare listening on port ${availablePort}`);

			devId++;
			startingPort = availablePort + 1;
		}

		for (const [miniflareDevId, miniflareData] of devIdToMiniflareInstanceMap) {
			const availablePort = await findAvailablePort(startingPort);

			const expressInstance = setExpressInstance(
				miniflareData.instance,
				miniflareData.port,
				availablePort,
			);

			devIdToExpressInstanceMap.set(miniflareDevId, {
				instance: expressInstance,
				port: availablePort,
			});

			log.info(`Express proxy listening on port ${availablePort}`);

			startingPort = availablePort + 1;
		}
		*/
	} catch (error) {
		//spin.stop();
		log.error(error);
	}
}

type ResourceDistFiles = string[];

/**
 * These are the dist files created after running
 * `npm` | `pnpm` `dev` at the project root.
 *
 * @example
 * ```ts
 * // given resource files of:
 * ['gasoline/core-base-api/index.core.base.api.ts']
 * // returns:
 * ['gasoline/core-base-api/dist/index.core.base.api.js']
 * ```
 */
function setResourceIndexDistFiles(
	resourceIndexFiles: ResourceIndexFiles,
): ResourceDistFiles {
	return resourceIndexFiles.map((resourceFile) =>
		path.join(
			resourceFile.replace(path.basename(resourceFile), ""),
			"dist",
			path.basename(resourceFile).replace(".ts", ".js"),
		),
	);
}

type ResourceDistFileExports = Record<string, unknown>[];

/**
 * @example
 * [
 *   {
 *     "coreBaseApiConfig": {
 *       "resource": "cloudflare-worker",
 *       "id": "core:base:cloudflare-worker:api:v1:12345",
 *       "domain": {
 *         "variant": "workers.dev"
 *       }
 *     },
 *     "default": {}
 *   }
 * ]
 */
async function getResourceDistFileExports(
	resourceDistFiles: ResourceIndexFiles,
): Promise<ResourceDistFileExports> {
	return Promise.all(
		resourceDistFiles.map(
			async (resourceDistFile) =>
				import(path.join(process.cwd(), resourceDistFile)) as Promise<
					Record<string, unknown>
				>,
		),
	);
}

type ResourceDistFileToConfigMap = Map<string, Record<string, unknown>>;

/**
 * @example
 * "gasoline/core-base-api/dist/index.core.base.api.js" => {
 *   resource: "cloudflare-worker",
 *   id: "core:base:cloudflare-worker:api:v1:12345",
 *   domain: {
 *     variant: "workers.dev"
 *   }
 * }
 */
function setResourceDistFileToConfigMap(
	resourceDistFiles: ResourceDistFiles,
	resourceDistFileExports: ResourceDistFileExports,
) {
	const result: ResourceDistFileToConfigMap = new Map();
	resourceDistFiles.forEach((resourceDistFile, index) => {
		for (const exportedItem in resourceDistFileExports[index]) {
			const config = resourceDistFileExports[index][exportedItem] as Record<
				string,
				unknown
			>;
			if (config.resource && config.id) {
				result.set(resourceDistFile, config);
			}
		}
	});
	return result;
}

type ResourceDistFileToDevIdMap = Map<string, number>;

function setResourceDistFileToDevIdMap(): ResourceDistFileToDevIdMap {
	return new Map();
}

function setMiniflareInstance(scriptPath: string, port: number) {
	return new Miniflare({
		port,
		modules: true,
		scriptPath,
	});
}

type DevIdToMiniflareMap = Map<
	number,
	{
		instance: Miniflare;
		port: number;
	}
>;

function setDevIdToMiniflareMap(): DevIdToMiniflareMap {
	return new Map();
}

function setExpressInstance(
	miniflare: Miniflare,
	miniflarePort: number,
	port: number,
) {
	const app = express();
	app.get("/", async (req, res) => {
		const fetchResponse = await miniflare.dispatchFetch(
			`http://localhost:${miniflarePort}/`,
		);
		if (fetchResponse.body) {
			Readable.fromWeb(fetchResponse.body).pipe(res);
		} else {
			res.status(500).send("Error: fetchResponse.body is null");
		}
	});
	app.listen(port);
	return app;
}

type DevIdToExpressInstanceMap = Map<
	number,
	{
		instance: Express;
		port: number;
	}
>;

function setDevIdToExpressInstanceMap(): DevIdToExpressInstanceMap {
	return new Map();
}

async function findAvailablePort(startPort: number): Promise<number> {
	let port = startPort;
	let isAvailable = false;

	while (!isAvailable) {
		try {
			await new Promise((resolve, reject) => {
				const testServer = http
					.createServer()
					.once("error", (err: NodeJS.ErrnoException) => {
						if (err.code === "EADDRINUSE") {
							resolve(false);
						} else {
							reject(err);
						}
					})
					.once("listening", () => {
						testServer.close(() => {
							isAvailable = true;
							resolve(true);
						});
					})
					.listen(port);
			});
		} catch (error) {
			throw new Error(
				`An error occurred while checking port availability: ${error}`,
			);
		}

		if (!isAvailable) {
			port++;
		}
	}

	return port;
}
