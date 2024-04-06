import { KVNamespace } from "@cloudflare/workers-types";

export type KvBindings<T extends ReadonlyArray<{ readonly binding: string }>> =
	{
		[P in T[number]["binding"]]: KVNamespace;
	};

type CloudflareKv = {
	id: string;
	name: string;
};

export function setCloudflareKv<T extends CloudflareKv>(resource: T): T {
	return resource;
}

type CloudflareWorker = {
	id: string;
	name: string;
	kv: Array<{
		binding: string;
	}>;
};

export function setCloudflareWorker<T extends CloudflareWorker>(
	resource: T,
): T {
	return resource;
}
