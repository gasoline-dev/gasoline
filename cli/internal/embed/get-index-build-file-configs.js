import { pathToFileURL } from 'url';
import path from 'path';

async function main() {
	try {
		const configs = await Promise.all(
			process.env.FILE_PATHS.split(',').map(async (filePath) => {
				const adjustedFilePath = path.join(process.cwd(), filePath);
				const fileURL = pathToFileURL(adjustedFilePath).href;
				const fileExports = await import(fileURL)
				for (const fileExport in fileExports) {
					const exportedItem = fileExports[fileExport];
					if (
						exportedItem.id &&
						/^[^:]*:[^:]*:[^:]*:[^:]*$/.test(exportedItem.id)
					) {
						return exportedItem;
					}
				}
			})
		);
		console.log(JSON.stringify(configs))
	} catch (error) {
		console.error('Error: unable to get exports')
    console.error(error);
	}
}

main();
