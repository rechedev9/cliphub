import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import { z } from "zod";

export type StudioMcpConfig = { command: string; args: string[] };

export type Config = {
  configFile: string | null;
  statePath: string;
  legacyMapsDir: string | null;
  legacyMetricsDir: string | null;
  studioMcp: StudioMcpConfig | null;
};

export type ConfigEnv = Readonly<Record<string, string | undefined>>;

export const ENV_PREFIX = "CLIPHUB_CONTENT_";

const PACKAGE_DIR = path.resolve(import.meta.dirname, "..");
const LOCAL_CONFIG_FILE = "config.local.json";

const fileSchema = z.strictObject({
  statePath: z.string().min(1).optional(),
  legacyMapsDir: z.string().min(1).optional(),
  legacyMetricsDir: z.string().min(1).optional(),
  studioMcp: z
    .strictObject({
      command: z.string().min(1),
      args: z.array(z.string()).default([]),
    })
    .optional(),
});

type ConfigFile = z.infer<typeof fileSchema>;

export class ConfigError extends Error {}

function envValue(env: ConfigEnv, name: string): string | null {
  const value = env[ENV_PREFIX + name]?.trim();
  return value ? value : null;
}

function readConfigFile(file: string): ConfigFile {
  let raw: unknown;
  try {
    raw = JSON.parse(readFileSync(file, "utf8"));
  } catch (error) {
    throw new ConfigError(`${file} is not valid JSON: ${String(error)}`);
  }
  const parsed = fileSchema.safeParse(raw);
  if (!parsed.success) {
    throw new ConfigError(`${file} is invalid: ${z.prettifyError(parsed.error)}`);
  }
  return parsed.data;
}

type Resolve = { base: string; env: ConfigEnv };

function expand(value: string, env: ConfigEnv): string {
  return value.replace(/%([A-Za-z_][A-Za-z0-9_]*)%/g, (_, name: string) => {
    const found = env[name]?.trim();
    if (!found) throw new ConfigError(`the config uses %${name}%, which is not set`);
    return found;
  });
}

function resolveFrom(at: Resolve, value: string | undefined): string | null {
  return value === undefined ? null : path.resolve(at.base, expand(value, at.env));
}

function resolveCommand(at: Resolve, command: string): string {
  const expanded = expand(command, at.env);
  return /[\\/]/.test(expanded) ? path.resolve(at.base, expanded) : expanded;
}

function defaultStatePath(env: ConfigEnv): string {
  const localAppData = env.LOCALAPPDATA?.trim() || path.join(homedir(), "AppData", "Local");
  return path.join(localAppData, "cliphub-content", "state.db");
}

export function loadConfig(options: { env: ConfigEnv; packageDir?: string }): Config {
  const { env } = options;
  const explicitFile = envValue(env, "CONFIG");
  const candidate = explicitFile ?? path.join(options.packageDir ?? PACKAGE_DIR, LOCAL_CONFIG_FILE);
  if (explicitFile !== null && !existsSync(explicitFile)) {
    throw new ConfigError(`${ENV_PREFIX}CONFIG points to ${explicitFile}, which does not exist`);
  }
  const configFile = existsSync(candidate) ? path.resolve(candidate) : null;
  const file: ConfigFile = configFile === null ? {} : readConfigFile(configFile);
  const at: Resolve = { base: configFile === null ? process.cwd() : path.dirname(configFile), env };

  const studioCommand = envValue(env, "STUDIO_MCP_COMMAND");
  const fileStudio = file.studioMcp
    ? { command: resolveCommand(at, file.studioMcp.command), args: file.studioMcp.args.map((arg) => expand(arg, env)) }
    : null;

  return {
    configFile,
    statePath: envValue(env, "STATE_PATH") ?? resolveFrom(at, file.statePath) ?? defaultStatePath(env),
    legacyMapsDir: envValue(env, "LEGACY_MAPS_DIR") ?? resolveFrom(at, file.legacyMapsDir),
    legacyMetricsDir: envValue(env, "LEGACY_METRICS_DIR") ?? resolveFrom(at, file.legacyMetricsDir),
    studioMcp: studioCommand === null ? fileStudio : { command: studioCommand, args: fileStudio?.args ?? [] },
  };
}
