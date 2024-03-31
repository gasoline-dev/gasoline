import { log, spin } from "../commons/cli.log.js";
import { CliParsedArgs } from "../index.cli.js";
import { getConfig } from "../commons/cli.config.js";
import {
	ResourceContainerDirs,
	getResourceIndexFiles,
	setResourceContainerDirs,
	setResourceDescriptor,
	setResourceEntityGroupEntities,
	setResourceEntityGroups,
} from "../commons/cli.resources.js";
import inquirer from "inquirer";
import path from "path";
import fsPromises from "fs/promises";
import {
	downloadTemplate,
	downloadTsConfigCloudflareWorkerJson,
} from "../commons/cli.templates.js";
import { isFilePresent, readJsonFile, renameFile } from "../commons/cli.fs.js";
import { PackageJson, getPackageManager } from "../commons/cli.packages.js";
import { promisify } from "util";
import { exec } from "node:child_process";
import { loadFile, writeFile } from "magicast";

export async function runAddCommand(
	cliCommand: string,
	cliParsedArgs: CliParsedArgs,
) {
	spin.start("Getting resources");
	try {
		const config = await getConfig();

		const resourceContainerDirs = setResourceContainerDirs(
			cliParsedArgs.values.resourceContainerDir,
			config.resourceContainerDirs,
		);

		spin.stop();

		let selectedResourceContainerDir = resourceContainerDirs[0];
		if (resourceContainerDirs.length > 1) {
			selectedResourceContainerDir = await runSetResourceContainerDirPrompt(
				resourceContainerDirs,
			);
		}

		spin.start("Getting resources");

		const resourceFiles = await getResourceIndexFiles([
			selectedResourceContainerDir,
		]);

		const resourceEntityGroups = setResourceEntityGroups(resourceFiles);

		spin.stop();

		let resourceEntityGroup = "";
		let resourceEntityGroupEntities = [];
		let resourceEntity = "";

		let resourceDnsZoneName = "";
		let resourceKvNamespace = "";

		switch (cliCommand) {
			case "add:cloudflare:dns:zone":
				resourceDnsZoneName = await runSetDnsZoneNamePrompt();
				break;
			case "add:cloudflare:kv":
				resourceKvNamespace = `${resourceEntityGroup
					.replace(/-/g, "_")
					.toUpperCase()}_${resourceEntity}_KV`;
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

		spin.start("Downloading template");
		await downloadTemplate(templateSrc, templateTargetDir);
		spin.stop();

		spin.start("Adjusting template");

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
			const mod = await loadFile(newTemplateIndexFileName);
			mod.exports.config.namespace = resourceKvNamespace;
			const camelCaseKvNamespace = resourceKvNamespace
				.split("-")
				.map(
					(part, index) =>
						part.charAt(0).toUpperCase() + part.slice(1).toLowerCase(),
				)
				.join("");
			mod.exports[`${camelCaseKvNamespace}KvNamespaceConfig`] =
				mod.exports.config;
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

		await fsPromises.writeFile(
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

		spin.stop();

		spin.start("Installing template packages");

		const packageManager = await getPackageManager();

		const promisifiedExec = promisify(exec);
		await promisifiedExec(`${packageManager} install`, {
			cwd: templateTargetDir,
		});

		spin.stop();

		log.info("Added template");
	} catch (error) {
		spin.stop();
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
