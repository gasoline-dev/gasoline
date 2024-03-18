import fsPromises from "fs/promises";
import { downloadTemplate as downloadTemplateFromGitHub } from "giget";
import path from "path";

/**
 * Download template from GitHub.
 */
export async function downloadTemplate(src: string, targetDir: string) {
	await downloadTemplateFromGitHub(src, {
		dir: targetDir,
		forceClean: true,
	});
}

/**
 * Download tsconfig-cloudflare-workers.json from GitHub.
 */
export async function downloadTsConfigCloudflareWorkerJson(targetDir: string) {
	const url =
		"https://raw.githubusercontent.com/gasoline-dev/gasoline/main/templates/tsconfig.cloudflare-workers.json";
	const response = await fetch(url);
	if (!response.ok) {
		throw new Error(`Failed to fetch ${url}: ${response.statusText}`);
	}
	const responseJson = await response.json();
	await fsPromises.writeFile(
		path.join(targetDir, "tsconfig.cloudflare-workers.json"),
		JSON.stringify(responseJson, null, 2),
	);
}
