type CloudflareKv = {
	id: string;
	namespace: string;
};

export function setCloudflareKv<T extends CloudflareKv>(resource: T): T {
	return resource;
}

type CloudflareWorker = {
	id: string;
	kv: Array<{
		binding: string;
	}>;
};

export function setCloudflareWorker<T extends CloudflareWorker>(
	resource: T,
): T {
	return resource;
}
