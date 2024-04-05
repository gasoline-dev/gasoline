import { Hono } from "hono";
import { setCloudflareWorker } from "@gasoline-dev/resources";

export const config = setCloudflareWorker({
	id: "core:base:cf-worker:api:v1:12345",
} as const);

type Bindings = {};

const app = new Hono<{ Bindings: Bindings }>();

app.get("/", (c) => c.text("Hello, World!"));

export default app;
