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

export async function runTurboInitCommand(cliParsedArgs: CliParsedArgs) {
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

				startingPort = availablePort + 1;
			}
		}
	} catch (error) {
		log.error(error);
	}
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
