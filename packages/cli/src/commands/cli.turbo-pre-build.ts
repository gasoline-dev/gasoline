import path from "node:path";
import { getConfig } from "../commons/cli.config.js";
import { log } from "../commons/cli.log.js";
import {
	getResourceIndexFiles,
	setResourceContainerDirs,
} from "../commons/cli.resources.js";
import { CliParsedArgs } from "../index.cli.js";
import fsPromises from "fs/promises";

export async function runTurboPreBuildCommand(cliParsedArgs: CliParsedArgs) {
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceIndexFiles = await getResourceIndexFiles(
			resourceContainerDirs,
		);

		for (const resourceIndexFile of resourceIndexFiles) {
			const splitResourceIndexFile = path
				.basename(resourceIndexFile)
				.split(".");
			const resourceEntityGroup = splitResourceIndexFile[0].replace("_", "");
			const resourceEntity = splitResourceIndexFile[1];
			const resourceDescriptor = splitResourceIndexFile[2];
			if (resourceDescriptor === "api") {
				const name = `${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`;
				const main = `src/${path.basename(resourceIndexFile)}`;
				const compatibilityDate = "2024-04-03";

				const wranglerBody = `name = "${name}"
main = "${main}"
compatibility_date = "${compatibilityDate}"
`;

				const resourceDir = path.dirname(path.dirname(resourceIndexFile));

				await fsPromises.writeFile(
					path.join(resourceDir, ".wrangler.toml"),
					wranglerBody,
				);
			}
		}
	} catch (error) {
		log.error(error);
	}
}
