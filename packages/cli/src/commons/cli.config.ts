import path from "path";
import { cwd } from "process";
import { isFilePresent } from "./cli.fs.js";

export type Config = {
	resourceContainerDirs?: [];
};

/**
 * Returns gasoline.config.js if it exists.
 */
export async function getConfig() {
	let result: Config = {};
	const configPath = "./gasoline.config.js";
	try {
		const isConfigPresent = await isFilePresent(configPath);
		if (!isConfigPresent) {
			return result;
		}
		const importedConfig = await import(path.join(cwd(), configPath));
		const configExport = importedConfig.default || module;
		result = {
			...configExport,
		};
		return result;
	} catch (error) {
		throw new Error(`Unable to get config: ${configPath}`);
	}
}
