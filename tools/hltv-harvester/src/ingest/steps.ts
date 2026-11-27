import { appendFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { storePaths, type StorePaths } from "../store.ts";
import type { IngestConfig } from "./config.ts";
import type { Runner } from "./runner.ts";

export type IngestContext = {
  cfg: IngestConfig;
  store: StorePaths;
  runner: Runner;
  log: (msg: string) => void;
  fetch?: typeof fetch;
};

export type ContextOptions = { cfg: IngestConfig; runner: Runner; print: (line: string) => void; fetch?: typeof fetch };

export function context({ cfg, runner, print, fetch: fetchImpl }: ContextOptions): IngestContext {
  const logFile = join(cfg.home, "ingest.log");
  const log = (msg: string): void => {
    const line = `${new Date().toISOString()} ingest ${msg}`;
    print(line);
    if (runner.dryRun) return;
    mkdirSync(cfg.home, { recursive: true });
    appendFileSync(logFile, `${line}\n`);
  };
  return { cfg, store: storePaths(cfg.home), runner, log, fetch: fetchImpl };
}

function alive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return err instanceof Error && "code" in err && err.code === "EPERM";
  }
}

export async function withLock(ctx: IngestContext, fn: () => Promise<number>): Promise<number> {
  const lock = join(ctx.cfg.home, "ingest.lock");
  if (ctx.runner.dryRun) {
    ctx.log(`[dry-run] take the lock ${lock}`);
    return fn();
  }
  mkdirSync(ctx.cfg.home, { recursive: true });
  if (existsSync(lock)) {
    const pid = Number(readFileSync(lock, "utf8").trim());
    if (Number.isInteger(pid) && pid > 0 && alive(pid)) {
      ctx.log(`class=busy message="another ingest run holds ${lock} (pid ${pid})"`);
      return 1;
    }
    rmSync(lock, { force: true });
  }
  try {
    writeFileSync(lock, String(process.pid), { flag: "wx" });
  } catch {
    ctx.log(`class=busy message="another ingest run took ${lock}"`);
    return 1;
  }
  try {
    return await fn();
  } finally {
    rmSync(lock, { force: true });
  }
}

export async function buildHarvester(ctx: IngestContext): Promise<void> {
  const res = await ctx.runner.run({ cmd: ctx.cfg.pnpm, args: ["run", "build"], cwd: ctx.cfg.harvesterDir, shell: true });
  if (res.code !== 0) ctx.log(`harvester build exited with ${res.code}`);
}

export function harvesterArgs(ctx: IngestContext, args: string[]): string[] {
  return [ctx.cfg.harvesterDir, ...args, "--out", ctx.cfg.home, "--community-url", ctx.cfg.communityUrl || "off"];
}

export async function prepare(ctx: IngestContext, keepDemos: boolean): Promise<boolean> {
  const args = ["prepare-hltv", "-downloads", ctx.store.downloads, "-out", ctx.store.analyses];
  if (keepDemos) args.push("-keep-demos", ctx.store.demos);
  const res = await ctx.runner.run({ cmd: ctx.cfg.communityExe, args, cwd: ctx.cfg.home });
  if (res.code !== 0) ctx.log(`prepare-hltv exited with ${res.code}`);
  return res.code === 0;
}

export function lines(text: string): Set<string> {
  return new Set(text.split(/\r?\n/).map((l) => l.trim()).filter((l) => l !== ""));
}

export function localAnalyses(store: StorePaths): string[] {
  return existsSync(store.analyses) ? readdirSync(store.analyses).sort() : [];
}

export async function remoteAnalyses(ctx: IngestContext): Promise<Set<string> | null> {
  const inbox = ctx.cfg.vpsInbox;
  const res = await ctx.runner.run({ cmd: "ssh", args: [ctx.cfg.sshHost, `ls ${inbox}/../analyses ${inbox}`] });
  if (res.code !== 0) {
    ctx.log(`listing the VPS analyses exited with ${res.code}`);
    return null;
  }
  return lines(res.stdout);
}

export async function uploadAnalyses(ctx: IngestContext, names: string[]): Promise<boolean> {
  if (names.length === 0) {
    ctx.log("nothing new");
    return true;
  }
  const { home } = ctx.cfg;
  const list = join(home, "new.txt");
  const tar = join(home, "upload.tar");
  const inbox = ctx.cfg.vpsInbox;
  ctx.runner.act(`write ${names.length} names to ${list}: ${names.join(", ")}`, () => writeFileSync(list, `${names.join("\n")}\n`));
  try {
    const packed = await ctx.runner.run({ cmd: "tar", args: ["cf", "../upload.tar", "-T", "../new.txt"], cwd: ctx.store.analyses });
    if (packed.code !== 0) {
      ctx.log(`packing the analyses exited with ${packed.code}`);
      return false;
    }
    const remote = `sudo tar xf - -C ${inbox} && sudo chown cliphub-community: ${inbox}/* && sudo chmod 0640 ${inbox}/* && sudo systemctl start cliphub-community-index.service`;
    const sent = await ctx.runner.run({ cmd: "ssh", args: [ctx.cfg.sshHost, remote], stdinFile: tar });
    if (sent.code !== 0) {
      ctx.log(`uploading the analyses exited with ${sent.code}`);
      return false;
    }
    ctx.log(`${ctx.runner.dryRun ? "would upload" : "uploaded"} ${names.length} analyses`);
    return true;
  } finally {
    ctx.runner.act(`remove ${tar}`, () => rmSync(tar, { force: true }));
  }
}
