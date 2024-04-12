import { pathToFileURL } from 'url';
import path from 'path';

async function main() {
  const filePath = path.join(process.cwd(), 'gas/core-base-kv/build/_core.base.kv.index.js');
  const fileURL = pathToFileURL(filePath).href;
  try {
		let resourceConfig = ''
    const moduleExports = await import(fileURL);
    for (const moduleExport in moduleExports) {
			const exportedModule = moduleExports[moduleExport];
			if (
				exportedModule.id &&
				/^[^:]*:[^:]*:[^:]*:[^:]*$/.test(exportedModule.id)
			) {
				resourceConfig = exportedModule;
			}
		}
		console.log(resourceConfig)
  } catch (error) {
		console.error('Error: unable to get exports')
    console.error(error);
  }
}

main();
