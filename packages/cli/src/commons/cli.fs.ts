import fsPromises from "fs/promises";

/**
 * Returns an array of dirs that exist in the given dir.
 */
export async function getDirs(dir: string) {
	const entries = await fsPromises.readdir(dir, {
		withFileTypes: true,
	});
	const result: string[] = [];
	for (const entry of entries) {
		if (entry.isDirectory()) {
			result.push(entry.name);
		}
	}
	return result;
}

type GetDirFilesOptions = {
	fileRegexToMatch?: RegExp;
};

/**
 * Returns an array of files that exist in the given dir.
 */
export async function getDirFiles(
	dir: string,
	options: GetDirFilesOptions = {},
) {
	const { fileRegexToMatch = /.*/ } = options;
	const result = [];
	const entries = await fsPromises.readdir(dir, {
		withFileTypes: true,
	});
	for (const entry of entries) {
		if (!entry.isDirectory() && fileRegexToMatch.test(entry.name)) {
			result.push(entry.name);
		}
	}
	return result;
}

/**
 * Returns true if a path is present, false if not.
 */
async function isPathPresent(pathToCheck: string) {
	try {
		await fsPromises.access(pathToCheck);
		return true;
	} catch (error) {
		return false;
	}
}

/**
 * Returns true if a file is present, false if not.
 */
export async function isFilePresent(file: string) {
	const pathIsPresent = await isPathPresent(file);
	if (pathIsPresent) {
		return true;
	}
	return false;
}

/**
 * Returns parsed JSON file.
 */
export async function readJsonFile<T extends Record<string, unknown>>(
	file: string,
): Promise<T> {
	const readFileResult = await fsPromises.readFile(file, "utf8");
	return JSON.parse(readFileResult);
}

/**
 * Renames given old file to given new file.
 */
export async function renameFile(oldPath: string, newPath: string) {
	await fsPromises.rename(oldPath, newPath);
}
