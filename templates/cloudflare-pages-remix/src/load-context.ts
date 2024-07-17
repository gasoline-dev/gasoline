/*
import { ServiceBindings } from "@gasoline-dev/resources";
import { type PlatformProxy } from "wrangler";
import { coreAppPages } from "./_core.app.pages.index";
*/

//type Bindings = ServiceBindings<(typeof coreAppPages)["services"]>;

//type Cloudflare = Omit<PlatformProxy<Bindings>, "dispose">;

// Will replace this: import { type PlatformProxy } from "wrangler";

interface Env {}

type Cloudflare = Env

// Will replace this: type Cloudflare = Omit<PlatformProxy<Env>, "dispose">;

declare module "@remix-run/cloudflare" {
	interface AppLoadContext {
		cloudflare: Cloudflare;
	}
}
