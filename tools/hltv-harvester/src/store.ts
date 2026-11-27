import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { parseState, type HarvestRecord, type HarvestState } from "./hltv.ts";
import type { RegistryReply } from "./registry.ts";

export type StorePaths = { home: string; downloads: string; analyses: string; demos: string; state: string };

export function storePaths(home: string): StorePaths {
  return {
    home,
    downloads: join(home, "downloads"),
    analyses: join(home, "analyses"),
    demos: join(home, "demos"),
    state: join(home, "harvest-state.json"),
  };
}

export function loadState(paths: StorePaths): HarvestState {
  if (!existsSync(paths.state)) return { matches: {} };
  return parseState(JSON.parse(readFileSync(paths.state, "utf8")));
}

export type MatchRecord = { paths: StorePaths; state: HarvestState; matchId: number; rec: Omit<HarvestRecord, "at"> };

export function recordMatch({ paths, state, matchId, rec }: MatchRecord): void {
  state.matches[String(matchId)] = { at: new Date().toISOString(), ...rec };
  mkdirSync(paths.home, { recursive: true });
  const tmp = `${paths.state}.tmp`;
  writeFileSync(tmp, JSON.stringify(state, null, 2));
  renameSync(tmp, paths.state);
}

function filesOf(dir: string, matchId: number, suffix: string): string[] {
  if (!existsSync(dir)) return [];
  const prefix = `${matchId}__`;
  return readdirSync(dir).filter((f) => f.startsWith(prefix) && f.endsWith(suffix)).sort();
}

export function archiveOf(paths: StorePaths, matchId: number): string {
  return join(paths.downloads, `${matchId}.rar`);
}

export function keptDemosOf(paths: StorePaths, matchId: number): string[] {
  return filesOf(paths.demos, matchId, ".dem");
}

export function analysesOf(paths: StorePaths, matchId: number): string[] {
  return filesOf(paths.analyses, matchId, ".zvpro.zst");
}

export function hasLocalDemo(paths: StorePaths, matchId: number): boolean {
  return existsSync(archiveOf(paths, matchId)) || keptDemosOf(paths, matchId).length > 0;
}

export type RegistrySplit<T> = { todo: T[]; registered: T[] };

export function splitRegistered<T extends { matchId: number }>(rows: T[], reply: RegistryReply): RegistrySplit<T> {
  if (!reply.ok) return { todo: rows, registered: [] };
  return { todo: rows.filter((r) => !reply.registered.has(r.matchId)), registered: rows.filter((r) => reply.registered.has(r.matchId)) };
}
