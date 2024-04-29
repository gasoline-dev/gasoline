import { pathToFileURL } from 'url';
import path from 'path';

async function main() {
	try {
		const resourceContainerSubdirPaths = process.env.RESOURCE_CONTAINER_SUBDIR_PATHS.split(',').map(subdirPath => subdirPath)

		const resourceIndexFilePaths = process.env.RESOURCE_INDEX_FILE_PATHS.split(',').map(filePath => filePath)

		const resourceIndexBuildFilePaths = process.env.RESOURCE_INDEX_BUILD_FILE_PATHS.split(',').map(buildFilePath => buildFilePath)

		const configs = await Promise.all(
			resourceContainerSubdirPaths.map(async (subdirPath, index) => {
				const resourceIndexBuildFilePath = resourceIndexBuildFilePaths[index]

				const adjustedResourceIndexBuildFilePath = path.join(process.cwd(), resourceIndexBuildFilePath);

				const resourceIndexBuildFileURL = pathToFileURL(adjustedResourceIndexBuildFilePath).href;

				const resourceIndexBuildFileExports = await import(resourceIndexBuildFileURL)

				const exportedConfigName = convertResourceContainerSubdirPathToCamelCase(subdirPath)

				if (!resourceIndexBuildFileExports[exportedConfigName]) {
					const resourceIndexFilePath = resourceIndexFilePaths[index]

					throw new Error(`resource ${resourceIndexFilePath} is required to have an exported config variable named ${exportedConfigName}`)
				}

				return resourceIndexBuildFileExports[exportedConfigName]
			})
		);

		console.log(JSON.stringify(configs))
	} catch (error) {
    console.error(error);
	}
}

main();

function convertResourceContainerSubdirPathToCamelCase(subdirPath) {
  const subdirName = path.basename(subdirPath);
  return subdirName.split('-').reduce((result, word, index) => {
    if (index === 0) {
      return word.toLowerCase();
    }
    return result + word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
  }, '');
}
