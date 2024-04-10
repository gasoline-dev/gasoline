#!/usr/bin/env node
import { parseArgs, promisify } from "node:util";
import {
	readFile,
	rename,
	writeFile as fsWriteFile,
	readdir,
	access,
} from "node:fs/promises";
import path from "node:path";
import { loadFile, writeFile } from "magicast";
import inquirer from "inquirer";
import { cwd } from "node:process";
import { downloadTemplate as downloadTemplateFromGitHub } from "giget";
import { exec } from "node:child_process";
import { Miniflare } from "miniflare";
import express, { Express } from "express";
import { Readable } from "node:stream";
import http from "http";
import { default as Ora } from "ora";

// INFO: Main

const cliOptions = {
	help: {
		type: "boolean",
		short: "h",
	},
	verbose: {
		type: "boolean",
		short: "v",
	},
	dir: {
		type: "string",
	},
	entityGroup: {
		type: "string",
	},
	resourceContainerDir: {
		type: "string",
	},
} as const;

const cliParsedArgs = parseArgs({
	allowPositionals: true,
	options: cliOptions,
});

export type CliParsedArgs = typeof cliParsedArgs;

if (cliParsedArgs.values.verbose) printVerboseLogs();

async function main() {
	try {
		const helpMessage = `Usage:
gasoline [command] -> Run command

Commands:
 add:cloudflare:dns:zone         Add Cloudflare DNS zone
 add:cloudflare:kv               Add Cloudflare KV storage
 add:cloudflare:worker:api:empty Add Cloudflare Worker API
 add:cloudflare:worker:api:hono  Add Cloudflare Worker Hono API
 deploy                          Deploy system to the cloud

Options:
 --help, -h Print help`;

		if (cliParsedArgs.positionals?.[0]) {
			const cliCommand = cliParsedArgs.positionals[0];

			const commandDoesNotExistMessage = `Command "${cliCommand}" does not exist. Run "gasoline --help" to see available commands.`;

			if (cliCommand.includes("add:")) {
				const availableAddCommands = [
					"add:cloudflare:dns:zone",
					"add:cloudflare:kv",
					"add:cloudflare:worker:api:empty",
					"add:cloudflare:worker:api:hono",
				];

				if (availableAddCommands.includes(cliCommand)) {
					await runAddCommand(cliCommand, cliParsedArgs);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else if (cliCommand === "deploy") {
				await runDeployCommand();
			} else if (cliCommand === "dev") {
				await runDevCommand(cliParsedArgs);
			} else if (cliCommand.includes("turbo:")) {
				const availableTurboCommands = [
					"turbo:init",
					"turbo:pre-build",
					"turbo:pre-dev",
				];

				if (
					availableTurboCommands.includes(cliCommand) &&
					cliCommand === "turbo:init"
				) {
					await runTurboInitCommand(cliParsedArgs);
				} else if (
					availableTurboCommands.includes(cliCommand) &&
					cliCommand === "turbo:pre-dev"
				) {
					await runTurboPreDevCommand(cliParsedArgs);
				} else {
					console.log(commandDoesNotExistMessage);
				}
			} else {
				console.log(commandDoesNotExistMessage);
			}
		} else {
			console.log(helpMessage);
		}
	} catch (error) {
		console.error(error);
	}
}

await main();

// INFO: Add command

async function runAddCommand(cliCommand: string, cliParsedArgs: CliParsedArgs) {
	//spin.start("Getting resources");
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		//spin.stop();

		let selectedResourceContainerDir = resourceContainerDirs[0];
		if (resourceContainerDirs.length > 1) {
			selectedResourceContainerDir = await runSetResourceContainerDirPrompt(
				resourceContainerDirs,
			);
		}

		//spin.start("Getting resources");

		const resourceFiles = await getResourceIndexFiles([
			selectedResourceContainerDir,
		]);

		const resourceEntityGroups = setResourceEntityGroups(resourceFiles);

		//spin.stop();

		let resourceEntityGroup = "";
		let resourceEntityGroupEntities = [];
		let resourceEntity = "";

		let resourceDnsZoneName = "";
		let resourceKvNamespace = "";

		switch (cliCommand) {
			case "add:cloudflare:dns:zone":
				resourceDnsZoneName = await runSetDnsZoneNamePrompt();
				break;
			default:
				resourceEntityGroup =
					await runSetResourceEntityGroupPrompt(resourceEntityGroups);
				resourceEntityGroupEntities =
					setResourceEntityGroupEntities(resourceFiles);
				resourceEntity = await runSetResourceEntityPrompt(
					resourceEntityGroupEntities,
				);
				break;
		}

		const resourceDescriptor = setResourceDescriptor(cliCommand);

		const templateSrc = `github:gasoline-dev/gasoline/templates/${cliCommand
			.replace("add:", "")
			.replace(/:/g, "-")}`;

		const templateTargetDir =
			cliCommand === "add:cloudflare:dns:zone"
				? path.join(
						selectedResourceContainerDir,
						`_${resourceDnsZoneName.replace(/\./g, "-")}-dns-zone`,
				  )
				: path.join(
						selectedResourceContainerDir,
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
				  );

		//spin.start("Downloading template");
		await downloadTemplate(templateSrc, templateTargetDir);
		//spin.stop();

		//spin.start("Adjusting template");

		const newTemplateIndexFileName =
			cliCommand === "add:cloudflare:dns:zone"
				? path.join(
						templateTargetDir,
						`src/_${resourceDnsZoneName.replace(/\./g, "-")}.dns.zone.index.ts`,
				  )
				: path.join(
						templateTargetDir,
						`src/_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.ts`,
				  );

		await renameFile(
			path.join(templateTargetDir, "src/index.ts"),
			newTemplateIndexFileName,
		);

		if (cliCommand === "add:cloudflare:dns:zone") {
			const mod = await loadFile(newTemplateIndexFileName);
			mod.exports.config.name = resourceDnsZoneName;
			const camelCaseDomain = resourceDnsZoneName
				.split(".")
				.map((part, index) =>
					part
						.split("-")
						.map((segment, segmentIndex) =>
							index === 0 && segmentIndex === 0
								? segment.toLowerCase()
								: segment.charAt(0).toUpperCase() +
								  segment.slice(1).toLowerCase(),
						)
						.join("-"),
				)
				.join("")
				.replaceAll("-", "");

			mod.exports[`${camelCaseDomain}DnsZoneConfig`] = mod.exports.config;
			// biome-ignore lint/performance/noDelete: magicast won't work without
			delete mod.exports.config;
			await writeFile(mod, newTemplateIndexFileName);
		}

		if (cliCommand === "add:cloudflare:kv") {
			resourceKvNamespace = `${resourceEntityGroup.replace(
				/-/g,
				"_",
			)}_${resourceEntity}_KV`.toUpperCase();
			const mod = await loadFile(newTemplateIndexFileName);
			mod.exports.config.namespace = resourceKvNamespace;
			mod.exports[
				`${resourceEntityGroup}${resourceEntity
					.charAt(0)
					.toUpperCase()}${resourceEntity.slice(1)}${resourceDescriptor
					.charAt(0)
					.toUpperCase()}${resourceDescriptor.slice(1)}Config`
			] = mod.exports.config;
			// biome-ignore lint/performance/noDelete: magicast won't work without
			delete mod.exports.config;
			await writeFile(mod, newTemplateIndexFileName);
		}

		const templatePackageJson = await readJsonFile<PackageJson>(
			path.join(templateTargetDir, "package.json"),
		);

		templatePackageJson.name =
			cliCommand === "add:cloudflare:dns:zone"
				? `${path.basename(
						selectedResourceContainerDir,
				  )}-${resourceDnsZoneName.replace(/\./g, "-")}-dns-zone`
				: `${path.basename(
						selectedResourceContainerDir,
				  )}-${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`;

		templatePackageJson.main =
			cliCommand === "add:cloudflare:dns:zone"
				? templatePackageJson.main.replace(
						"z.z.z.index.js",
						`_${resourceDnsZoneName.replace(/\./g, "-")}.dns.zone.index.js`,
				  )
				: templatePackageJson.main.replace(
						"z.z.z.index.js",
						`_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.js`,
				  );

		templatePackageJson.types =
			cliCommand === "add:cloudflare:dns:zone"
				? templatePackageJson.types.replace(
						"z.z.z.index.d.ts",
						`_${resourceDnsZoneName.replace(/\./g, "-")}.dns.zone.index.d.ts`,
				  )
				: templatePackageJson.types.replace(
						"z.z.z.index.d.ts",
						`_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.d.ts`,
				  );

		templatePackageJson.scripts.build =
			cliCommand === "add:cloudflare:dns:zone"
				? templatePackageJson.scripts.build.replace(
						"z.z.z.index.ts",
						`_${resourceDnsZoneName.replace(/\./g, "-")}.dns.zone.index.ts`,
				  )
				: templatePackageJson.scripts.build.replace(
						"z.z.z.index.ts",
						`_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.ts`,
				  );

		templatePackageJson.scripts.dev =
			cliCommand === "add:cloudflare:dns:zone"
				? templatePackageJson.scripts.dev.replace(
						"z.z.z.index.ts",
						`_${resourceDnsZoneName.replace(/\./g, "-")}.dns.zone.index.ts`,
				  )
				: templatePackageJson.scripts.dev.replace(
						"z.z.z.index.ts",
						`_${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.index.ts`,
				  );

		await fsWriteFile(
			path.join(templateTargetDir, "package.json"),
			JSON.stringify(templatePackageJson, null, 2),
		);

		if (cliCommand.includes("cloudflare:worker")) {
			const tsConfigCloudflareWorkerJsonIsPresent = await isFilePresent(
				path.join(
					selectedResourceContainerDir,
					"tsconfig.cloudflare-workers.json",
				),
			);

			if (!tsConfigCloudflareWorkerJsonIsPresent) {
				await downloadTsConfigCloudflareWorkerJson(
					selectedResourceContainerDir,
				);
			}
		}

		//spin.stop();

		//spin.start("Installing template packages");

		const packageManager = await getPackageManager();

		const promisifiedExec = promisify(exec);
		await promisifiedExec(`${packageManager} install`, {
			cwd: templateTargetDir,
		});

		//spin.stop();

		log.info("Added template");
	} catch (error) {
		//spin.stop();
		log.error(error);
	}
}

async function runSetResourceContainerDirPrompt(
	resolvedResourceContainerDirPaths: ResourceContainerDirs,
) {
	const { resourceContainerDir } = await inquirer.prompt([
		{
			type: "list",
			name: "resourceContainerDir",
			message: "Select resource container dir",
			choices: resolvedResourceContainerDirPaths,
		},
	]);
	return resourceContainerDir;
}

async function runSelectEntityGroupPrompt(resourceEntityGroups: string[]) {
	const { resourceEntityGroup } = await inquirer.prompt([
		{
			type: "list",
			name: "resourceEntityGroup",
			message: "Select resource entity group",
			choices: ["Add new", ...resourceEntityGroups],
		},
	]);
	return resourceEntityGroup;
}

async function runAddResourceEntityGroupPrompt() {
	const { resourceEntityGroup } = await inquirer.prompt([
		{
			type: "input",
			name: "resourceEntityGroup",
			message: "Enter resource entity group",
		},
	]);
	return resourceEntityGroup;
}

async function runSetResourceEntityGroupPrompt(resourceEntityGroups: string[]) {
	let result = "";
	if (resourceEntityGroups.length > 0) {
		result = await runSelectEntityGroupPrompt(resourceEntityGroups);
	} else {
		result = await runAddResourceEntityGroupPrompt();
	}
	if (result === "Add new") {
		result = await runAddResourceEntityGroupPrompt();
	}
	return result;
}

async function runSelectResourceEntityPrompt(resourceEntities: string[]) {
	const { resourceEntity } = await inquirer.prompt([
		{
			type: "list",
			name: "resourceEntity",
			message: "Select resource entity",
			choices: ["Add new", ...resourceEntities],
		},
	]);
	return resourceEntity;
}

async function runAddResourceEntity() {
	const { resourceEntity } = await inquirer.prompt([
		{
			type: "input",
			name: "resourceEntity",
			message: "Enter resource entity",
		},
	]);
	return resourceEntity;
}

async function runSetResourceEntityPrompt(resourceEntities: string[]) {
	if (resourceEntities.length === 0) {
		return await runAddResourceEntity();
	}
	let result = await runSelectResourceEntityPrompt(resourceEntities);
	if (result === "Add new") {
		result = await runAddResourceEntity();
	}
	return result;
}

async function runSetDnsZoneNamePrompt() {
	const { dnsZoneName } = await inquirer.prompt([
		{
			type: "input",
			name: "dnsZoneName",
			message: "Enter DNS zone name (example.com)",
			validate: (input) => {
				const domainRegex =
					/^(?=.{1,253}$)([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,6}(\.[a-zA-Z]{2,6})?$/;
				if (!domainRegex.test(input)) {
					return "Needs to be a valid domain";
				}
				return true;
			},
		},
	]);
	return dnsZoneName.toLowerCase();
}

async function runSetKvNamespacePrompt() {
	const { kvName } = await inquirer.prompt([
		{
			type: "input",
			name: "kvName",
			message: "Enter KV namespace name",
			validate: (input) => {
				const kvNameRegex = /^[a-zA-Z0-9_]{1,64}$/;
				if (!kvNameRegex.test(input)) {
					return "Needs to be a valid KV namespace name";
				}
				return true;
			},
		},
	]);
	return kvName.toLowerCase();
}

// INFO: Deploy command

async function runDeployCommand() {
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceContainerDir = resourceContainerDirs[0];

		const resourceIndexFiles = await getResourceIndexFiles([
			resourceContainerDir,
		]);

		const resourceIndexDistFiles =
			setResourceIndexDistFiles(resourceIndexFiles);

		const resourceExports = await getResourceExports(resourceIndexDistFiles);

		const resourceDirs = await getResourceDirs(resourceContainerDirs);

		const resourcePackageJsons = await getResourcePackageJsons(resourceDirs);

		const resourcePackageJsonNamesSet =
			setResourcePackageJsonNamesSet(resourcePackageJsons);

		const packageJsonNameToResourceIdMap = setPackageJsonNameToResourceIdMap(
			resourcePackageJsons,
			resourceExports,
		);

		const resourceDependencies = setResourceDependencies(
			resourcePackageJsons,
			packageJsonNameToResourceIdMap,
			resourcePackageJsonNamesSet,
		);

		//const resourceMap = setResourceMap(resourceExports, resourceDependencies);

		//console.log(resourceMap);

		//const test = resourceMap.get("test");
		//if (test && test.type === "cloudflare-pages") {
		//console.log(test.exports.services);
		//}

		// await deploy({}, resourceManifest);
	} catch (error) {
		log.error(error);
	}
}

async function deploy(prevResourceManifest: any, currResourceManifest: any) {}

// INFO: Dev command

export async function runDevCommand(cliParsedArgs: CliParsedArgs) {
	//spin.start("Getting resources");
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		const resourceIndexFiles = await getResourceIndexFiles(
			resourceContainerDirs,
		);

		// TODO: copy types
		/*
			// _core.base.kv.index.d.ts
			export type KvConfig = {
				type: "cloudflare-kv";
				id: string;
				namespace: string;
			};

			export const coreBaseKvConfig: KvConfig;
		*/

		let startingPort = 8787;
		const resourceToPortMap = new Map<string, number>();
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

				await fsWriteFile(
					path.join(resourceDir, ".wrangler.toml"),
					wranglerBody,
				);

				resourceToPortMap.set(
					`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
					availablePort,
				);

				startingPort = availablePort + 1;
			}
		}

		//const watcher = chokidar.watch(
		//[
		//"gasoline/*/.wrangler/tmp/dev-**/*.*.*.index.js",
		//"gasoline/*/dist/*.*.*.index.js",
		//],
		//	{
		//	ignoreInitial: true,
		//	persistent: true,
		//},
		//);

		/*
		watcher
			// TODO: add
			//.on("add", async (watchedPath) => {
			//console.log(`File ${watchedPath} has been added`);
			//})
			.on("change", async (watchedPath) => {
				console.log(`File ${watchedPath} has been changed`);

				if (watchedPath.includes(".wrangler")) {
					const splitWatchedPath = path.normalize(watchedPath).split(path.sep);

					const resourceContainerDir = splitWatchedPath[0];

					const splitWatchedBasename = path.basename(watchedPath).split(".");

					const resourceEntityGroup = splitWatchedBasename[0].replace("_", "");
					const resourceEntity = splitWatchedBasename[1];
					const resourceDescriptor = splitWatchedBasename[2];

					const resourceIndexTsFile = path.join(
						resourceContainerDir,
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
						"src",
						path.basename(watchedPath).replace(".js", ".ts"),
					);

					const moduleImports = (await import(
						path.join(process.cwd(), watchedPath)
					)) as Record<string, any>;

					const availablePort = resourceToPortMap.get(
						`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
					);
					const name = `${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`;
					const main = `src/${path.basename(resourceIndexTsFile)}`;

					let wranglerBody = `name = "${name}"
main = "${main}"

[dev]
port = ${availablePort}

`;

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
							resourceContainerDir,
							`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
							".wrangler.toml",
						),
						wranglerBody,
					);
				}
			});
		// TODO: delete
		// .on("unlink", (path) => console.log(`File ${path} has been removed`));
		*/

		/*
		const packageManager = await getPackageManager();

		const devAllProcess = spawn(packageManager, ["run", "dev:all"], {
			shell: true,
			stdio: "inherit",
		});

		devAllProcess.on("exit", (code) => {
			console.log(`Child process exited with code ${code}`);
		});
		*/

		return;

		/*
		const resourceDistFiles = setResourceIndexDistFiles(resourceIndexFiles);

		const resourceDistFileExports =
			await getResourceDistFileExports(resourceDistFiles);

		const resourceDistFileToConfigMap = setResourceDistFileToConfigMap(
			resourceDistFiles,
			resourceDistFileExports,
		);

		const resourceDistFileToDevIdMap = setResourceDistFileToDevIdMap();

		const devIdToMiniflareInstanceMap = setDevIdToMiniflareMap();

		const devIdToExpressInstanceMap = setDevIdToExpressInstanceMap();
		*/

		//spin.stop();

		/*
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
		*/
	} catch (error) {
		//spin.stop();
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
export function setResourceIndexDistFiles(
	resourceIndexFiles: ResourceIndexFiles,
): ResourceDistFiles {
	return resourceIndexFiles.map((resourceFile) =>
		path.join(
			resourceFile.replace("/src", "").replace(path.basename(resourceFile), ""),
			"dist",
			path.basename(resourceFile).replace(".ts", ".js"),
		),
	);
}

export type ResourceExports = any[];

/**
 * @example
 * [
 *   {
 *     "id": "core:base:cloudflare-worker:api:v1:12345",
 *     "domain": {
 *       "variant": "workers.dev"
 *     }
 *   }
 * ]
 */
export async function getResourceExports(
	resourceDistFiles: ResourceIndexFiles,
): Promise<ResourceExports> {
	return Promise.all(
		resourceDistFiles.map((resourceDistFile) =>
			import(path.join(process.cwd(), resourceDistFile)).then((fileExports) => {
				for (const fileExport in fileExports) {
					const exportedItem = fileExports[fileExport];
					if (
						exportedItem.id &&
						/^[^:]*:[^:]*:[^:]*:[^:]*$/.test(exportedItem.id)
					) {
						return exportedItem;
					}
				}
			}),
		),
	);
}

/*
export async function getResourceIndexDistFileExports(
	resourceDistFiles: ResourceIndexFiles,
): Promise<ResourceIndexDistFileExports> {
	return Promise.all(
		resourceDistFiles.map(
			(resourceDistFile) =>
				import(path.join(process.cwd(), resourceDistFile)) as Promise<
					Record<string, unknown>
				>,
		),
	);
}
*/

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
	resourceDistFileExports: ResourceExports,
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

// INFO: Turbo init command

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

				await fsWriteFile(
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

// INFO: Turbo pre-build command

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

				await fsWriteFile(
					path.join(resourceDir, ".wrangler.toml"),
					wranglerBody,
				);
			}
		}
	} catch (error) {
		log.error(error);
	}
}

// INFO: Turbo prev-dev command

export async function runTurboPreDevCommand(cliParsedArgs: CliParsedArgs) {
	try {
		async function readWranglerToml() {
			const wranglerToml = await readFile(
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

			await fsWriteFile(
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

// INFO: Config

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

// INFO: Filesystem

/**
 * Returns an array of dirs that exist in the given dir.
 */
export async function getDirs(dir: string) {
	const entries = await readdir(dir, {
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
	const entries = await readdir(dir, {
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
		await access(pathToCheck);
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
	const readFileResult = await readFile(file, "utf8");
	return JSON.parse(readFileResult);
}

/**
 * Renames given old file to given new file.
 */
export async function renameFile(oldPath: string, newPath: string) {
	await rename(oldPath, newPath);
}

// INFO: Logs

type LoggerOptions = {
	initialLevel?: LoggerLevel;
	mode?: LoggerMode;
};

type LoggerLevel = "trace" | "debug" | "info" | "warn" | "error" | "fatal";

type LoggerMode = "json" | "pretty";

type LogBody = string | Record<string, unknown>;

type LogError = unknown;

function Logger(options: LoggerOptions = {}) {
	const { initialLevel = "trace", mode = "json" } = options;

	const redColorCode = "\x1b[31m";
	const resetColorCode = "\x1b[0m";

	const levels = {
		trace: 10,
		debug: 20,
		info: 30,
		warn: 40,
		error: 50,
		fatal: 60,
	};

	let minLevel = levels[initialLevel];

	function log(body: LogBody, level: LoggerLevel) {
		if (levels[level] < minLevel) return;
		console.log(body);
	}

	function logError(error: LogError) {
		if (levels.error < minLevel) return;

		if (error instanceof Error) {
			if (mode === "pretty") {
				console.error(`${redColorCode}ERROR${resetColorCode} ${error.stack}`);
			} else {
				const errorJson = {
					level: 50,
					time: new Date().toISOString(),
					message: error.message,
					name: error.name,
					stack: error.stack,
				};
				console.error(JSON.stringify(errorJson));
			}
		} else {
			console.error(`${redColorCode}ERROR${resetColorCode} ${String(error)} `);
		}
	}

	function setLevel(level: LoggerLevel) {
		minLevel = levels[level];
	}

	return {
		trace: (body: LogBody) => log(body, "trace"),
		debug: (body: LogBody) => log(body, "debug"),
		info: (body: LogBody) => log(body, "info"),
		warn: (body: LogBody) => log(body, "warn"),
		error: (error: LogError) => logError(error),
		fatal: (body: LogBody) => log(body, "fatal"),
		setLevel,
	};
}

const logger = Logger({
	initialLevel: "info",
	mode: "pretty",
});

let ora = Ora();

export function printVerboseLogs() {
	logger.setLevel("trace");

	ora = Ora({
		isSilent: true,
	});
}

export const log = {
	trace(body: LogBody) {
		logger.trace(body);
	},
	debug(body: LogBody) {
		logger.debug(body);
	},
	info(body: LogBody) {
		logger.info(body);
	},
	warn(body: LogBody) {
		logger.warn(body);
	},
	error(body: unknown) {
		logger.error(body);
	},
	fatal(body: LogBody) {
		logger.fatal(body);
	},
};

export const spin = {
	fail(msg: string) {
		ora.fail(msg);
	},
	start(msg: string) {
		ora.start(msg);
	},
	stop() {
		ora.stop();
	},
	succeed(msg: string) {
		ora.succeed(msg);
	},
};

// INFO: Package manager

export type PackageJson = {
	name: string;
	main: string;
	types: string;
	scripts: {
		build: string;
		dev: string;
	};
	dependencies?: Record<string, string>;
	devDependencies?: Record<string, string>;
};

type PackageManager = "npm" | "pnpm";

/**
 * Returns the project's package manager.
 */
export async function getPackageManager(): Promise<PackageManager> {
	const packageManagers: Array<PackageManager> = ["npm", "pnpm"];
	for (const packageManager of packageManagers) {
		switch (packageManager) {
			case "npm": {
				const isPackageLockJsonPresent =
					await isFilePresent("package-lock.json");
				if (isPackageLockJsonPresent) {
					return packageManager;
				}
				break;
			}
			case "pnpm": {
				const isPnpmLockYamlPresent = await isFilePresent("pnpm-lock.yaml");
				if (isPnpmLockYamlPresent) {
					return packageManager;
				}
				break;
			}
		}
	}
	throw new Error("No supported package manager found (npm or pnpm)");
}

// INFO: Resources

export type ResourceDirs = string[];

/**
 * Returns an array of resource dirs.
 *
 * @example
 * ```ts
 * ['gasoline/core-base-api']
 * ```
 */
export async function getResourceDirs(
	resourceContainerDirs: string[],
): Promise<ResourceDirs> {
	const resourceDirs = await Promise.all(
		resourceContainerDirs.map(async (resourceContainerDir) => {
			const getDirsResult = await getDirs(resourceContainerDir);
			return getDirsResult.map((dir) => `${resourceContainerDir}/${dir}`);
		}),
	);
	return resourceDirs.flat();
}

export type ResourceIndexFiles = string[];

/**
 * @example
 * ```ts
 * ['gasoline/core-base-api/src/_core.base.api.index.ts']
 * ```
 */
export async function getResourceIndexFiles(
	resourceContainerDirs: ResourceContainerDirs,
): Promise<ResourceIndexFiles> {
	const resourceDirs = await getResourceDirs(resourceContainerDirs);
	const resourceIndexFiles = await Promise.all(
		resourceDirs.map(async (resourceDir) => {
			const getDirFilesResult = await getDirFiles(`${resourceDir}/src`, {
				fileRegexToMatch: /^_[^.]+\.[^.]+\.[^.]+\.[^.]+\.[^.]+$/,
			});
			return getDirFilesResult.map((file) => `${resourceDir}/src/${file}`);
		}),
	);
	return resourceIndexFiles.flat();
}

export type ResourceContainerDirs = string[];

/**
 * Returns an array of resource container dirs.
 *
 * It returns an array so projects can have multiple if necessary.
 *
 * Most projects will have one: `gasoline`.
 *
 * @example
 * ```ts
 * ['gasoline']
 * ```
 */
export function setResourceContainerDirs(
	cliResourceContainerDir: CliParsedArgs["values"]["resourceContainerDir"],
	configResourceContainerDirs: Config["resourceContainerDirs"],
) {
	let result: ResourceContainerDirs = [];
	if (cliResourceContainerDir) {
		result = [path.relative(process.cwd(), cliResourceContainerDir)];
	} else if (configResourceContainerDirs) {
		result = configResourceContainerDirs.map((dir) =>
			path.relative(process.cwd(), dir),
		);
	} else {
		result = ["gasoline"];
	}
	return result;
}

type ResourceEntityGroups = string[];

/**
 * Returns an array of resource entity groups.
 *
 * @example
 * ```ts
 * // given resource files of:
 * ['gasoline/core-base-api/_core.base.api.index.ts']
 * // return:
 * ['core']
 * ```
 */
export function setResourceEntityGroups(
	resourceIndexFiles: ResourceIndexFiles,
) {
	const result: ResourceEntityGroups = [];
	for (const resourceIndexFile of resourceIndexFiles) {
		if (!resourceIndexFile.includes(".dns.zone.")) {
			const group = path.basename(resourceIndexFile).split(".")[0];
			if (!result.includes(group)) result.push(group.replace("_", ""));
		}
	}
	return result;
}

type ResourceEntityGroupEntities = string[];

/**
 * @example
 * ```ts
 * // given resource files of:
 * ['gasoline/core-base-api/_core.base.api.index.ts']
 * // return:
 * ['base']
 * ```
 */
export function setResourceEntityGroupEntities(
	resourceIndexFiles: ResourceIndexFiles,
) {
	const result: ResourceEntityGroupEntities = [];
	for (const resourceIndexFile of resourceIndexFiles) {
		if (!resourceIndexFile.includes(".dns.zone.")) {
			const entity = path.basename(resourceIndexFile).split(".")[1];
			if (!result.includes(entity)) result.push(entity);
		}
	}
	return result;
}

/**
 * Returns a resource descriptor.
 *
 * A resource descriptor is the last part of a resource file. It
 * describes what the resource is. (e.g. `api` is the resource
 * descriptor for `gasoline/core-base-api/src/index.core.base.api.ts`).
 *
 * @example
 * ```ts
 * // given a cli command of:
 * "add:cloudflare:worker:api:hono"
 * // return:
 * "api"
 * ```
 */
export function setResourceDescriptor(cliCommand: string) {
	if (cliCommand === "add:cloudflare:dns:zone") return "zone";
	if (cliCommand === "add:cloudflare:kv") return "kv";
	if (cliCommand === "add:cloudflare:worker:api:empty") return "api";
	if (cliCommand === "add:cloudflare:worker:api:hono") return "api";
	throw new Error(
		`Resource descriptor cannot be set for CLI command: ${cliCommand}`,
	);
}

type ResourcePackageJsons = Array<PackageJson>;

/**
 * Returns an array of parsed resource package.json files.
 *
 * @example
 * ```ts
 * [
 *   {
 *     "name": "core-base-api",
 *     ...
 *   }
 * ]
 * ```
 */
async function getResourcePackageJsons(
	resourceDirs: ResourceDirs,
): Promise<ResourcePackageJsons> {
	const resourcePackageJsons = await Promise.all(
		resourceDirs.map(async (resourceDir) => {
			const packageJson = await readFile(
				path.join(resourceDir, "package.json"),
				"utf-8",
			);
			return JSON.parse(packageJson);
		}),
	);
	return resourcePackageJsons;
}

type ResourcePackageJsonNamesSet = Set<string>;

/**
 * Returns a resource package.json names set.
 *
 * package.json names are derived from each resource's
 * package.json name property.
 *
 * @example
 * ```ts
 * { 'core-base-api' }
 * ```
 */
function setResourcePackageJsonNamesSet(
	resourcePackageJsons: ResourcePackageJsons,
): ResourcePackageJsonNamesSet {
	const result = new Set<string>();
	for (const packageJson of resourcePackageJsons) {
		result.add(packageJson.name);
	}
	return result;
}

type PackageJsonNameToResourceIdMap = Map<string, string>;

/**
 * Returns a `package.json` name to resource ID map.
 *
 * Resource relationships are managed via each resource's
 * `package.json`. For example, package `core-base-kv` might
 * be a dependency of package `core-base-api`. Therefore,
 * `core-base-kv` would exist in `core-base-api's` `package.json's`
 * _`dependencies`_.
 *
 * When `core-base-api's` `package.json` is processed and the
 * `core-base-kv` dependency is found, this map can look up `core-base-kv's` resource ID. Thus, establishing that resource
 * `core:base:cloudflare-kv:12345` is a dependency of
 * `core:base:cloudflare-worker:12345`.
 *
 * @example
 * ```ts
 * {
 *   'core-base-api' => 'core:base:cloudflare-worker:12345',
 *   'core-base-kv' => 'core:base:cloudflare-kv:12345'
 * }
 * ```
 */
function setPackageJsonNameToResourceIdMap(
	resourcePackageJsons: ResourcePackageJsons,
	resourceExports: ResourceExports,
): PackageJsonNameToResourceIdMap {
	const result = new Map<string, string>();
	for (const [index, packageJson] of resourcePackageJsons.entries()) {
		result.set(packageJson.name, resourceExports[index].id);
	}
	return result;
}

type ResourceDependencies = Array<Array<string>>;

/**
 * Returns an array of resource dependencies.
 *
 * Resource dependencies are resources a resource depends on.
 * For example, resource `core:base:cloudflare-worker:12345`
 * might depend on `core:base:cloudflare-kv:12345`.
 *
 * @example
 * ```
 * // index 0 is core:base:cloudflare-worker:12345
 * // index 1 is core:base:cloudflare-kv:12345
 * // core:base:cloudflare-worker:12345 depends on
 * // core:base:cloudflare-kv:12345, while
 * // core:base:cloudflare-kv:12345 depends on nothing.
 * [ [ 'core:base:cloudflare-kv:12345' ], [] ]
 * ```
 */
function setResourceDependencies(
	resourcPackageJsons: ResourcePackageJsons,
	packageJsonNameToResourceIdMap: PackageJsonNameToResourceIdMap,
	resourcePackageJsonNamesSet: ResourcePackageJsonNamesSet,
): ResourceDependencies {
	const result: ResourceDependencies = [];
	for (const [index, packageJson] of resourcPackageJsons.entries()) {
		const dependencies = Object.keys(packageJson.dependencies ?? {});
		const internalDependencies: Array<string> = [];
		for (const dependency of dependencies) {
			const resourceId = packageJsonNameToResourceIdMap.get(dependency);
			if (
				resourceId !== undefined &&
				resourcePackageJsonNamesSet.has(dependency)
			) {
				internalDependencies.push(resourceId);
			}
		}
		result.push(internalDependencies);
	}
	return result;
}

/*
type ResourceMap = Map<
	string,
	(
		| {
				type: "cloudflare-kv";
				exports: CloudflareKv;
		  }
		| {
				type: "cloudflare-pages";
				exports: CloudflarePages;
		  }
		| {
				type: "cloudflare-worker";
				exports: CloudflareWorker;
		  }
	) & { dependsOn: Array<string> }
>;

function setResourceMap(
	resourceExports: ResourceExports,
	resourceDependencies: ResourceDependencies,
): ResourceMap {
	const result: ResourceMap = new Map();
	for (const [index, exports] of resourceExports.entries()) {
		result.set(exports.id, {
			//@ts-ignore
			// Resources should be validated before getting here.
			type: exports.id.split(":")[2],
			exports,
			dependsOn: resourceDependencies[index],
		});
	}
	return result;
}
*/

// INFO: Templates

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
	await fsWriteFile(
		path.join(targetDir, "tsconfig.cloudflare-workers.json"),
		JSON.stringify(responseJson, null, 2),
	);
}
