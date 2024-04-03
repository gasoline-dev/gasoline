import path from "node:path";
import { getConfig } from "../commons/cli.config.js";
import { log } from "../commons/cli.log.js";
import {
	getResourceIndexFiles,
	setResourceContainerDirs,
} from "../commons/cli.resources.js";
import { CliParsedArgs } from "../index.cli.js";
import fsPromises from "fs/promises";
import http from "http";

export async function runTurboPreDevCommand(cliParsedArgs: CliParsedArgs) {
	try {
		async function readWranglerToml() {
			const wranglerToml = await fsPromises.readFile(
				path.join(process.cwd(), ".wrangler.toml"),
				"utf8",
			);
			return wranglerToml;
		}

		let wranglerBody = await readWranglerToml();

		const splitResourceDir = path.basename(process.cwd()).split("-");
		const resourceEntityGroup = splitResourceDir[0];
		const resourceEntity = splitResourceDir[1];
		const resourceDescriptor = splitResourceDir[2];

		if (resourceDescriptor === "api") {
			const moduleImports = (await import(
				path.join(
					process.cwd(),
					"dist",
					`_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.js`,
				)
			)) as Record<string, any>;

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
								wranglerBody += `
[[kv_namespaces]]
binding = "${item.binding}"
id = "<GAS_DEV_PLACEHOLDER>"
	`;
							}
						}
					}
				}
			}

			await fsPromises.writeFile(
				path.join(process.cwd(), ".wrangler.toml"),
				wranglerBody,
			);
		}
	} catch (error) {
		log.error(error);
	}

	/*
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
				const compatibilityDate = "2024-04-03";

				let wranglerBody = `name = "${name}"
main = "${main}"
compatibility_date = "${compatibilityDate}"

[dev]
port = ${availablePort}
`;

				// const resourceDir = path.dirname(path.dirname(resourceIndexFile));

				const moduleImports = (await import(
					path.join(
						process.cwd(),
						"gasoline",
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
						"dist",
						`_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.js`,
					)
				)) as Record<string, any>;

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
						"gasoline",
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
						".wrangler.toml",
					),
					wranglerBody,
				);

				startingPort = availablePort + 1;
			}
		}
	} catch (error) {
		log.error(error);
	}
	*/
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
