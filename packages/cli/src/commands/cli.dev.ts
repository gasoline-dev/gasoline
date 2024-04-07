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
		console.log("ran");
		return;

		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceIndexFiles = await getResourceIndexFiles(
			resourceContainerDirs,
		);

		// TODO: copy types
		/*
			// _core.base.kv.index.d.ts
			export type KvConfig = {
				type: "cloudflare-kv";
				id: string;
				namespace: string;
			};

			export const coreBaseKvConfig: KvConfig;
		*/

		let startingPort = 8787;
		const resourceToPortMap = new Map<string, number>();
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
				const compatibilityDate = "2024-04-03";

				const wranglerBody = `name = "${name}"
main = "${main}"
compatibility_date = "${compatibilityDate}"

[dev]
port = ${availablePort}
`;

				const resourceDir = path.dirname(path.dirname(resourceIndexFile));

				await fsPromises.writeFile(
					path.join(resourceDir, ".wrangler.toml"),
					wranglerBody,
				);

				resourceToPortMap.set(
					`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
					availablePort,
				);

				startingPort = availablePort + 1;
			}
		}

		//const watcher = chokidar.watch(
		//[
		//"gasoline/*/.wrangler/tmp/dev-**/*.*.*.index.js",
		//"gasoline/*/dist/*.*.*.index.js",
		//],
		//	{
		//	ignoreInitial: true,
		//	persistent: true,
		//},
		//);

		/*
		watcher
			// TODO: add
			//.on("add", async (watchedPath) => {
			//console.log(`File ${watchedPath} has been added`);
			//})
			.on("change", async (watchedPath) => {
				console.log(`File ${watchedPath} has been changed`);

				if (watchedPath.includes(".wrangler")) {
					const splitWatchedPath = path.normalize(watchedPath).split(path.sep);

					const resourceContainerDir = splitWatchedPath[0];

					const splitWatchedBasename = path.basename(watchedPath).split(".");

					const resourceEntityGroup = splitWatchedBasename[0].replace("_", "");
					const resourceEntity = splitWatchedBasename[1];
					const resourceDescriptor = splitWatchedBasename[2];

					const resourceIndexTsFile = path.join(
						resourceContainerDir,
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
						"src",
						path.basename(watchedPath).replace(".js", ".ts"),
					);

					const moduleImports = (await import(
						path.join(process.cwd(), watchedPath)
					)) as Record<string, any>;

					const availablePort = resourceToPortMap.get(
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
					);
					const name = `${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`;
					const main = `src/${path.basename(resourceIndexTsFile)}`;

					let wranglerBody = `name = "${name}"
main = "${main}"

[dev]
port = ${availablePort}

`;

					for (const moduleImport in moduleImports) {
						if (
							moduleImports[moduleImport].type &&
							moduleImports[moduleImport].id
						) {
							const type = moduleImports[moduleImport].type;
							const kv = moduleImports[moduleImport].kv;
							if (type === "cloudflare-worker") {
								if (kv) {
									for (const item of kv) {
										wranglerBody += `[[kv_namespaces]]
binding = "${item.binding}"
id = "<GAS_DEV_PLACEHOLDER>"
`;
									}
								}
							}
						}
					}

					await fsPromises.writeFile(
						path.join(
							resourceContainerDir,
							`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
							".wrangler.toml",
						),
						wranglerBody,
					);
				}
			});
		// TODO: delete
		// .on("unlink", (path) => console.log(`File ${path} has been removed`));
		*/

		/*
		const packageManager = await getPackageManager();

		const devAllProcess = spawn(packageManager, ["run", "dev:all"], {
			shell: true,
			stdio: "inherit",
		});

		devAllProcess.on("exit", (code) => {
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
export function setResourceIndexDistFiles(
	resourceIndexFiles: ResourceIndexFiles,
): ResourceDistFiles {
	return resourceIndexFiles.map((resourceFile) =>
		path.join(
			resourceFile.replace("/src", "").replace(path.basename(resourceFile), ""),
			"dist",
			path.basename(resourceFile).replace(".ts", ".js"),
		),
	);
}

type ResourceIndexDistFileExportedConfigs = any[];

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
export async function getResourceIndexDistFileExportedConfigs(
	resourceDistFiles: ResourceIndexFiles,
): Promise<ResourceIndexDistFileExportedConfigs> {
	return Promise.all(
		resourceDistFiles.map((resourceDistFile) =>
			import(path.join(process.cwd(), resourceDistFile)).then((fileExports) => {
				for (const fileExport in fileExports) {
					const exportedItem = fileExports[fileExport];
					if (
						exportedItem.id &&
						/^[^:]*:[^:]*:[^:]*:[^:]*$/.test(exportedItem.id)
					) {
						return exportedItem;
					}
				}
			}),
		),
	);
}

/*
export async function getResourceIndexDistFileExports(
	resourceDistFiles: ResourceIndexFiles,
): Promise<ResourceIndexDistFileExports> {
	return Promise.all(
		resourceDistFiles.map(
			(resourceDistFile) =>
				import(path.join(process.cwd(), resourceDistFile)) as Promise<
					Record<string, unknown>
				>,
		),
	);
}
*/

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
	resourceDistFileExports: ResourceIndexDistFileExportedConfigs,
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
