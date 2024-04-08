import { isFilePresent } from "./cli.fs.js";

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
