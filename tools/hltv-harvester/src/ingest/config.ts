import { existsSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { homedir } from "node:os";
import { join, resolve } from "node:path";
import { communityURL } from "../registry.ts";

export type IngestConfig = {
  home: string;
  repo: string;
  harvesterDir: string;
  electron: string;
  pnpm: string;
  communityExe: string;
  communityUrl: string;
  sshHost: string;
  vpsInbox: string;
  vpsClips: string;
  keepDemos: boolean;
  harvestArgs: string[];
};

const KEYS = ["home", "repo", "electron", "pnpm", "communityUrl", "sshHost", "vpsInbox", "vpsClips"] as const;

const ENV: Record<(typeof KEYS)[number], string> = {
  home: "CLIPHUB_HLTV_HOME",
  repo: "CLIPHUB_REPO",
  electron: "CLIPHUB_ELECTRON",
  pnpm: "CLIPHUB_PNPM",
  communityUrl: "CLIPHUB_COMMUNITY_URL",
  sshHost: "CLIPHUB_VPS_SSH",
  vpsInbox: "CLIPHUB_VPS_INBOX",
  vpsClips: "CLIPHUB_VPS_CLIPS",
};

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function readFile(path: string): Record<string, unknown> {
  if (path === "") return {};
  if (!existsSync(path)) throw new Error(`no config file at ${path}`);
  const raw: unknown = JSON.parse(readFileSync(path, "utf8"));
  if (!isRecord(raw)) throw new Error(`${path} is not a JSON object`);
  return raw;
}

function electronPath(): string {
  try {
    const found: unknown = createRequire(import.meta.url)("electron");
    return typeof found === "string" ? found : "electron";
  } catch {
    return "electron";
  }
}

export function loadConfig(env: NodeJS.ProcessEnv, configFile: string): IngestConfig {
  const file = readFile(configFile || env.CLIPHUB_INGEST_CONFIG || "");
  const pick = (key: (typeof KEYS)[number]): string => {
    const fromEnv = env[ENV[key]];
    if (fromEnv !== undefined && fromEnv !== "") return fromEnv;
    const fromFile = file[key];
    return typeof fromFile === "string" ? fromFile : "";
  };
  const harvesterDir = resolve(import.meta.dirname, "..", "..");
  const repo = pick("repo") || resolve(harvesterDir, "..", "..");
  const home = pick("home") || join(homedir(), "cliphub-hltv");
  const extra = file.harvestArgs;
  return {
    home,
    repo,
    harvesterDir,
    electron: pick("electron") || electronPath(),
    pnpm: pick("pnpm") || "pnpm",
    communityExe: join(home, process.platform === "win32" ? "cliphub-community.exe" : "cliphub-community"),
    communityUrl: communityURL(pick("communityUrl"), {}),
    sshHost: pick("sshHost") || "openclaw",
    vpsInbox: pick("vpsInbox") || "/var/lib/cliphub-community/hltv",
    vpsClips: pick("vpsClips") || "/var/lib/cliphub-community/clips",
    keepDemos: file.keepDemos === true,
    harvestArgs: Array.isArray(extra) && extra.every((a) => typeof a === "string") ? extra : ["--days", "7", "--top", "50", "--ranked-only"],
  };
}
