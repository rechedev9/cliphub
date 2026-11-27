import { existsSync, mkdirSync, readdirSync, renameSync, rmSync, statSync, writeFileSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { buildHarvester, harvesterArgs, lines, localAnalyses, prepare, remoteAnalyses, uploadAnalyses, withLock, type IngestContext } from "./steps.ts";

const PARSER = "github.com/rechedev9/demo-parser-rsn";
const CLIP_BATCH = 100;

export function daily(ctx: IngestContext): Promise<number> {
  return withLock(ctx, async () => {
    ctx.log("== start");
    await buildHarvester(ctx);
    const harvested = await ctx.runner.run({ cmd: ctx.cfg.electron, args: harvesterArgs(ctx, ctx.cfg.harvestArgs), cwd: ctx.cfg.harvesterDir });
    if (harvested.code !== 0) ctx.log(`harvester exited with ${harvested.code}`);
    if (!(await buildCommunity(ctx))) ctx.log("parser update failed, keeping the current cliphub-community");
    if (!(await prepare(ctx, ctx.cfg.keepDemos))) return 1;
    const remote = await remoteAnalyses(ctx);
    if (remote === null) return 1;
    if (!(await uploadAnalyses(ctx, localAnalyses(ctx.store).filter((f) => !remote.has(f))))) return 1;
    if (!(await syncClips(ctx))) ctx.log("clips sync failed");
    ctx.log("== done");
    return 0;
  });
}

export async function buildCommunity(ctx: IngestContext): Promise<boolean> {
  const { home, repo, communityExe } = ctx.cfg;
  const src = join(home, ".build-src");
  const tar = join(home, ".build-src.tar");
  const next = `${communityExe}.new`;
  const { runner } = ctx;
  const clean = (): void => runner.act(`remove ${src}, ${tar} and ${next}`, () => {
    rmSync(src, { recursive: true, force: true });
    rmSync(tar, { force: true });
    rmSync(next, { force: true });
  });
  clean();
  runner.act(`create ${src}`, () => mkdirSync(src, { recursive: true }));
  try {
    const steps = [
      { cmd: "git", args: ["-C", repo, "fetch", "-q", "origin", "main"] },
      { cmd: "git", args: ["-C", repo, "archive", "-o", tar, "origin/main"] },
      { cmd: "tar", args: ["xf", "../.build-src.tar"], cwd: src },
      { cmd: "go", args: ["build", "-o", next, "./services/community"], cwd: src },
    ];
    for (const step of steps) if ((await runner.run(step)).code !== 0) return false;
    runner.act(`replace ${communityExe} with the new build`, () => renameSync(next, communityExe));
    const rev = await runner.run({ cmd: "git", args: ["-C", repo, "rev-parse", "--short", "origin/main"] });
    const version = await runner.run({ cmd: "go", args: ["list", "-m", "-f", "{{.Version}}", PARSER], cwd: src });
    ctx.log(`parser ${PARSER} ${version.stdout.trim()}, cliphub main ${rev.stdout.trim()}`);
    return true;
  } finally {
    clean();
  }
}

export function localClips(dir: string): string[] {
  if (!existsSync(dir)) return [];
  const out: string[] = [];
  const walk = (d: string): void => {
    for (const e of readdirSync(d, { withFileTypes: true })) {
      const full = join(d, e.name);
      if (e.isDirectory()) walk(full);
      else if (e.isFile() && e.name.endsWith(".mp4") && !e.name.endsWith(".tmp.mp4") && !e.name.endsWith(".re.mp4")) {
        out.push(`${relative(dir, full).split(sep).join("/")} ${statSync(full).size}`);
      }
    }
  };
  walk(dir);
  return out.sort();
}

export async function syncClips(ctx: IngestContext): Promise<boolean> {
  const { runner, cfg } = ctx;
  const clips = join(cfg.home, "clips");
  const remoteDir = cfg.vpsClips;
  const remote = await runner.run({ cmd: "ssh", args: [cfg.sshHost, `sudo find ${remoteDir} -name '*.mp4' -printf '%P %s\\n'`] });
  if (remote.code !== 0) return false;
  const have = lines(remote.stdout);
  const fresh = localClips(clips).filter((l) => !have.has(l)).map((l) => l.slice(0, l.lastIndexOf(" ")));
  if (fresh.length === 0) {
    ctx.log("clips: nothing new");
    return true;
  }
  const list = join(cfg.home, "clips-batch.txt");
  const tar = join(cfg.home, "clips-batch.tar");
  try {
    for (let i = 0; i < fresh.length; i += CLIP_BATCH) {
      const batch = fresh.slice(i, i + CLIP_BATCH);
      runner.act(`write ${batch.length} clip names to ${list}`, () => writeFileSync(list, `${batch.join("\n")}\n`));
      if ((await runner.run({ cmd: "tar", args: ["cf", "../clips-batch.tar", "-T", "../clips-batch.txt"], cwd: clips })).code !== 0) return false;
      if ((await runner.run({ cmd: "ssh", args: [cfg.sshHost, `sudo tar xf - --no-same-owner -C ${remoteDir}`], stdinFile: tar })).code !== 0) return false;
    }
  } finally {
    runner.act(`remove ${list} and ${tar}`, () => {
      rmSync(list, { force: true });
      rmSync(tar, { force: true });
    });
  }
  const perms = `sudo chown -R cliphub-community: ${remoteDir} && sudo find ${remoteDir} -type d -exec chmod 0750 {} + && sudo find ${remoteDir} -type f -exec chmod 0640 {} +`;
  if ((await runner.run({ cmd: "ssh", args: [cfg.sshHost, perms] })).code !== 0) return false;
  ctx.log(`clips: uploaded ${fresh.length}`);
  const lineups = "systemctl is-active --quiet cliphub-community-index.service || sudo systemd-run --wait --pipe --quiet -p User=cliphub-community -p Group=cliphub-community -p EnvironmentFile=/etc/cliphub-community/community.env -p MemoryMax=1536M /opt/cliphub-community/cliphub-community build-lineups 2>&1 | tail -1";
  return (await runner.run({ cmd: "ssh", args: [cfg.sshHost, lineups] })).code === 0;
}
