import { registeredMatches } from "../registry.ts";
import { analysesOf, hasLocalDemo } from "../store.ts";
import { buildHarvester, harvesterArgs, prepare, remoteAnalyses, uploadAnalyses, withLock, type IngestContext } from "./steps.ts";

export function ingestMatch(ctx: IngestContext, matchId: number): Promise<number> {
  return withLock(ctx, async () => {
    ctx.log(`== match ${matchId} start`);
    if (hasLocalDemo(ctx.store, matchId)) {
      ctx.log(`match ${matchId} is already in ${ctx.store.home}, no download`);
    } else {
      await buildHarvester(ctx);
      const res = await ctx.runner.run({ cmd: ctx.cfg.electron, args: harvesterArgs(ctx, ["--match", String(matchId)]), cwd: ctx.cfg.harvesterDir });
      if (res.code !== 0 || (!ctx.runner.dryRun && !hasLocalDemo(ctx.store, matchId))) {
        ctx.log(`the harvester saved no demo for match ${matchId} (exit ${res.code})`);
        return 1;
      }
    }
    if (!(await prepare(ctx, true))) return 1;
    const reply = await registeredMatches({ base: ctx.cfg.communityUrl, ids: [matchId], fetch: ctx.fetch });
    if (reply.ok && reply.registered.has(matchId)) {
      ctx.log(`match ${matchId} is already registered on the VPS, no upload`);
      return 0;
    }
    if (!reply.ok) ctx.log(`class=registry_unavailable error="${reply.error}" message="the VPS listing decides what to upload"`);
    const remote = await remoteAnalyses(ctx);
    if (remote === null) return 1;
    const local = analysesOf(ctx.store, matchId);
    const names = local.length === 0 && ctx.runner.dryRun ? [`${matchId}__*.zvpro.zst`] : local.filter((f) => !remote.has(f));
    if (!ctx.runner.dryRun && local.length === 0) {
      ctx.log(`prepare-hltv wrote no analysis for match ${matchId}`);
      return 1;
    }
    if (!(await uploadAnalyses(ctx, names))) return 1;
    ctx.log(`== match ${matchId} done`);
    return 0;
  });
}
