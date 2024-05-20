import { Fetcher, KVNamespace } from "@cloudflare/workers-types";

export type Resources = CloudflareKv | CloudflarePages | CloudflareWorker;

export type KvBindings<T extends ReadonlyArray<{ readonly binding: string }>> =
	{
		[P in T[number]["binding"]]: KVNamespace;
	};

export type ServiceBindings<
	T extends ReadonlyArray<{ readonly binding: string }>,
> = {
	[P in T[number]["binding"]]: Fetcher;
};

export type CloudflareKv = {
	name: string;
};

export function cloudflareKv<T extends CloudflareKv>(resource: T): T {
	return resource;
}

export type CloudflarePages = {
	id: string;
	name: string;
	services?: Array<{
		binding: string;
	}>;
};

export function setCloudflarePages<T extends CloudflarePages>(resource: T): T {
	return resource;
}

export type CloudflareWorker = {
	id: string;
	name: string;
	kv?: Array<{
		binding: string;
	}>;
};

export function setCloudflareWorker<T extends CloudflareWorker>(
	resource: T,
): T {
	return resource;
}
