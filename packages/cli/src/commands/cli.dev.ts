import path from "path";
import http from "http";
import { getConfig } from "../commons/cli.config.js";
import { log, spin } from "../commons/cli.log.js";
import {
	ResourceFiles,
	getResourceFiles,
	setResourceContainerDirs,
} from "../commons/cli.resources.js";
import { CliParsedArgs } from "../index.cli.js";
import express, { Express } from "express";
import { Miniflare } from "miniflare";
import { Readable } from "stream";

export async function runDevCommand(cliParsedArgs: CliParsedArgs) {
	spin.start("Getting resources");
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceFiles = await getResourceFiles(resourceContainerDirs);

		const resourceDistFiles = setResourceDistFiles(resourceFiles);

		const resourceDistFileExports =
			await getResourceDistFileExports(resourceDistFiles);

		const resourceDistFileToConfigMap = setResourceDistFileToConfigMap(
			resourceDistFiles,
			resourceDistFileExports,
		);

		const resourceDistFileToDevIdMap = setResourceDistFileToDevIdMap();

		const devIdToMiniflareInstanceMap = setDevIdToMiniflareMap();

		const devIdToExpressInstanceMap = setDevIdToExpressInstanceMap();

		spin.stop();

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
	} catch (error) {
		spin.stop();
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
function setResourceDistFiles(resourceFiles: ResourceFiles): ResourceDistFiles {
	return resourceFiles.map((resourceFile) =>
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
	resourceDistFiles: ResourceFiles,
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
