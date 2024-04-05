import { setCloudflareKv } from "@gasoline-dev/resources";

export const coreBaseKv = setCloudflareKv({
	id: "core:base:cloudflare-kv:v1:12345",
	namespace: "",
} as const);
