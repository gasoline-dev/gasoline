import { default as Ora } from "ora";

type LoggerOptions = {
	initialLevel?: LoggerLevel;
	mode?: LoggerMode;
};

type LoggerLevel = "trace" | "debug" | "info" | "warn" | "error" | "fatal";

type LoggerMode = "json" | "pretty";

type LogBody = string | Record<string, unknown>;

type LogError = unknown;

function Logger(options: LoggerOptions = {}) {
	const { initialLevel = "trace", mode = "json" } = options;

	const redColorCode = "\x1b[31m";
	const resetColorCode = "\x1b[0m";

	const levels = {
		trace: 10,
		debug: 20,
		info: 30,
		warn: 40,
		error: 50,
		fatal: 60,
	};

	let minLevel = levels[initialLevel];

	function log(body: LogBody, level: LoggerLevel) {
		if (levels[level] < minLevel) return;
		console.log(body);
	}

	function logError(error: LogError) {
		if (levels.error < minLevel) return;

		if (error instanceof Error) {
			if (mode === "pretty") {
				console.error(`${redColorCode}ERROR${resetColorCode} ${error.stack}`);
			} else {
				const errorJson = {
					level: 50,
					time: new Date().toISOString(),
					message: error.message,
					name: error.name,
					stack: error.stack,
				};
				console.error(JSON.stringify(errorJson));
			}
		} else {
			console.error(`${redColorCode}ERROR${resetColorCode} ${String(error)} `);
		}
	}

	function setLevel(level: LoggerLevel) {
		minLevel = levels[level];
	}

	return {
		trace: (body: LogBody) => log(body, "trace"),
		debug: (body: LogBody) => log(body, "debug"),
		info: (body: LogBody) => log(body, "info"),
		warn: (body: LogBody) => log(body, "warn"),
		error: (error: LogError) => logError(error),
		fatal: (body: LogBody) => log(body, "fatal"),
		setLevel,
	};
}

const logger = Logger({
	initialLevel: "info",
	mode: "pretty",
});

let ora = Ora();

export function printVerboseLogs() {
	logger.setLevel("trace");

	ora = Ora({
		isSilent: true,
	});
}

export const log = {
	trace(body: LogBody) {
		logger.trace(body);
	},
	debug(body: LogBody) {
		logger.debug(body);
	},
	info(body: LogBody) {
		logger.info(body);
	},
	warn(body: LogBody) {
		logger.warn(body);
	},
	error(body: unknown) {
		logger.error(body);
	},
	fatal(body: LogBody) {
		logger.fatal(body);
	},
};

export const spin = {
	fail(msg: string) {
		ora.fail(msg);
	},
	start(msg: string) {
		ora.start(msg);
	},
	stop() {
		ora.stop();
	},
	succeed(msg: string) {
		ora.succeed(msg);
	},
};
