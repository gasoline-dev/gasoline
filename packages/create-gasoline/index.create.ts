#!/usr/bin/env node
import path from "node:path";
import fsPromises from "fs/promises";
import inquirer from "inquirer";
import { promisify } from "node:util";
import { exec } from "node:child_process";
import { parseArgs } from "node:util";
import { downloadTemplate } from "giget";

await main();

async function main() {
	try {
		const options = {
			help: {
				type: "boolean",
				short: "h",
			},
		} as const;

		const parsedArgs = parseArgs({
			allowPositionals: true,
			options,
		});

		const helpMessage = `Usage:
create-gasoline -> Initalize project

Options:
	--help, -h Print help`;

		if (
			parsedArgs.positionals.length === 0 &&
			Object.keys(parsedArgs.values).length === 0
		) {
			await createProject();
		} else {
			console.log(helpMessage);
		}
	} catch (error) {
		console.error(error);
	}
}

async function createProject() {
	async function runSetDirPrompt() {
		try {
			const { dir } = await inquirer.prompt([
				{
					name: "dir",
					message: "Directory path:",
					default: "./example",
				},
			]);
			return dir;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to set directory path");
		}
	}

	async function checkIfPathIsPresent(path: string) {
		try {
			await fsPromises.access(path);
			return true;
		} catch (error) {
			return false;
		}
	}

	async function checkIfDirIsPresent(dir: string) {
		try {
			console.log(`Checking if ${dir} is present`);
			const isDirPresent = await checkIfPathIsPresent(dir);
			if (!isDirPresent) {
				console.log(`${dir} is not present`);
				return false;
			}
			console.log(`${dir} is present`);
			return true;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to check if ${dir} is present`);
		}
	}

	async function getDirContents(dir: string) {
		try {
			console.log(`Getting ${dir} contents`);
			const contents = await fsPromises.readdir(dir);
			console.log(`Got ${dir} contents`);
			return contents;
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to get ${dir} contents`);
		}
	}

	async function runEmptyDirContentsConfirmPrompt() {
		try {
			const { confirm } = await inquirer.prompt([
				{
					type: "confirm",
					name: "confirm",
					message: "Directory is not empty. Empty it?",
					default: false,
				},
			]);
			return confirm;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to confirm if directory should be emptied");
		}
	}

	async function emptyDirContents(dir: string) {
		try {
			console.log(`Emptying ${dir} contents`);
			const contents = await fsPromises.readdir(dir);
			await Promise.all(
				contents.map((file) => {
					return fsPromises.rm(path.join(dir, file), {
						recursive: true,
					});
				}),
			);
			console.log(`Emptied ${dir} contents`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to empty ${dir} contents`);
		}
	}

	async function runSetPackageManagerPrompt() {
		try {
			const { packageManager } = await inquirer.prompt([
				{
					type: "list",
					name: "packageManager",
					message: "Package manager:",
					choices: ["npm", "pnpm"],
					default: "npm",
				},
			]);
			return packageManager;
		} catch (error) {
			console.error(error);
			throw new Error("Unable to set package manager");
		}
	}

	async function downloadMonorepoPnpmTemplate(dir: string) {
		try {
			console.log("Downloading monorepo pnpm template");
			await downloadTemplate(
				"github:gasoline-dev/gasoline/templates/create-gasoline-monorepo-pnpm",
				{
					dir,
				},
			);
			console.log("Downloaded monorepo pnpm template");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to download monorepo pnpm template");
		}
	}

	async function downloadMonorepoNpmTemplate(dir: string) {
		try {
			console.log("Downloading monorepo npm template");
			await downloadTemplate(
				"github:gasoline-dev/gasoline/templates/create-gasoline-monorepo-npm",
				{
					dir,
				},
			);
			console.log("Downloaded monorepo npm template");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to download monorepo npm template");
		}
	}

	async function installPackages(packageManager: "npm" | "pnpm", dir: string) {
		try {
			console.log("Installing packages");
			const promisifiedExec = promisify(exec);
			await promisifiedExec(`${packageManager} install`, {
				cwd: path.resolve(dir),
			});
			console.log("Installed packages");
		} catch (error) {
			console.error(error);
			throw new Error("Unable to install packages");
		}
	}

	async function deleteFile(file: string) {
		try {
			console.log(`Deleting ${file}`);
			await fsPromises.rm(file);
			console.log(`Deleted ${file}`);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to delete ${file}`);
		}
	}

	type PackageJson = {
		name: string;
	};

	async function getPackageJson(file: string): Promise<PackageJson> {
		try {
			console.log(`Getting ${file}`);
			const packageJson = await fsPromises.readFile(
				path.join(process.cwd(), file),
				"utf-8",
			);
			console.log(`Got ${file}`);
			return JSON.parse(packageJson);
		} catch (error) {
			console.error(error);
			throw new Error(`Unable to get ${file}`);
		}
	}

	async function run() {
		const dir = await runSetDirPrompt();

		const isDirPresent = await checkIfDirIsPresent(dir);

		if (isDirPresent) {
			const dirContents = await getDirContents(dir);

			if (dirContents.length > 0) {
				const confirmEmptyDirContents =
					await runEmptyDirContentsConfirmPrompt();

				if (confirmEmptyDirContents) {
					await emptyDirContents(dir);
				} else {
					console.log(
						"create-gasoline is for new projects and requires an empty directory",
					);
					process.exit(0);
				}
			}
		}

		const packageManager = await runSetPackageManagerPrompt();

		if (packageManager === "pnpm") {
			await downloadMonorepoPnpmTemplate(dir);
		} else {
			await downloadMonorepoNpmTemplate(dir);
		}

		const packageJson = await getPackageJson(path.join(dir, "package.json"));

		packageJson.name = "root";

		await deleteFile(path.join(dir, "gasoline/.gitkeep"));

		await installPackages(packageManager, dir);
	}

	try {
		await run();
	} catch (error) {
		console.error(error);
		throw new Error("Unable to create project");
	}
}
