import { log, spin } from "../commons/cli.log.js";
import { CliParsedArgs } from "../index.cli.js";
import { getConfig } from "../commons/cli.config.js";
import {
	ResourceContainerDirs,
	getResourceFiles,
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

		const resourceFiles = await getResourceFiles([
			selectedResourceContainerDir,
		]);

		const resourceEntityGroups = setResourceEntityGroups(resourceFiles);

		spin.stop();

		const resourceEntityGroup =
			await runSetResourceEntityGroupPrompt(resourceEntityGroups);

		const resourceEntityGroupEntities =
			setResourceEntityGroupEntities(resourceFiles);

		const resourceEntity = await runSetResourceEntityPrompt(
			resourceEntityGroupEntities,
		);

		const resourceDescriptor = setResourceDescriptor(cliCommand);

		const templateSrc = `github:gasoline-dev/gasoline/templates/${cliCommand
			.replace("add:", "")
			.replace(/:/g, "-")}`;

		const templateTargetDir = path.join(
			selectedResourceContainerDir,
			`${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`,
		);

		spin.start("Downloading template");
		await downloadTemplate(templateSrc, templateTargetDir);
		spin.stop();

		spin.start("Adjusting template");

		await renameFile(
			path.join(templateTargetDir, "src/index.ts"),
			path.join(
				templateTargetDir,
				`src/index.${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.ts`,
			),
		);

		const templatePackageJson = await readJsonFile<PackageJson>(
			path.join(templateTargetDir, "package.json"),
		);

		templatePackageJson.name = `${path.basename(
			selectedResourceContainerDir,
		)}-${resourceEntityGroup}-${resourceEntity}-${resourceDescriptor}`;

		templatePackageJson.scripts.build =
			templatePackageJson.scripts.build.replace(
				"index.x.x.x.ts",
				`index.${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.ts`,
			);

		templatePackageJson.scripts.dev = templatePackageJson.scripts.dev.replace(
			"index.x.x.x.ts",
			`index.${resourceEntityGroup}.${resourceEntity}.${resourceDescriptor}.ts`,
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
