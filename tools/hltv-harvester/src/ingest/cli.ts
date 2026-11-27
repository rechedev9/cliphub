import { pathToFileURL } from "node:url";
import { loadConfig } from "./config.ts";
import { daily } from "./daily.ts";
import { ingestMatch } from "./match.ts";
import { dryRunner, spawnRunner, type Runner } from "./runner.ts";
import { context } from "./steps.ts";

export const USAGE = "usage: ingest daily [--dry-run] [--config FILE] | ingest match <hltv match id> [--dry-run] [--config FILE]";

export type CliDeps = {
  env: NodeJS.ProcessEnv;
  print: (line: string) => void;
  runner?: Runner;
  fetch?: typeof fetch;
};

type Parsed = { command: "daily" } | { command: "match"; matchId: number };

function parse(argv: string[]): { parsed: Parsed | null; dryRun: boolean; config: string } {
  const rest: string[] = [];
  let dryRun = false;
  let config = "";
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === "--dry-run") dryRun = true;
    else if (argv[i] === "--config") config = argv[++i] ?? "";
    else if (argv[i] !== "--") rest.push(argv[i] ?? "");
  }
  const [command, id, ...extra] = rest;
  if (extra.length > 0) return { parsed: null, dryRun, config };
  if (command === "daily" && id === undefined) return { parsed: { command }, dryRun, config };
  const matchId = Number(id);
  if (command === "match" && Number.isInteger(matchId) && matchId > 0) return { parsed: { command, matchId }, dryRun, config };
  return { parsed: null, dryRun, config };
}

export async function main(argv: string[], deps: CliDeps): Promise<number> {
  const { parsed, dryRun, config } = parse(argv);
  if (parsed === null) {
    deps.print(USAGE);
    return 2;
  }
  let cfg;
  try {
    cfg = loadConfig(deps.env, config);
  } catch (err) {
    deps.print(String(err));
    return 2;
  }
  const runner = deps.runner ?? (dryRun ? dryRunner(deps.print) : spawnRunner(deps.print));
  const ctx = context({ cfg, runner, print: deps.print, fetch: deps.fetch });
  return parsed.command === "daily" ? daily(ctx) : ingestMatch(ctx, parsed.matchId);
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await main(process.argv.slice(2), { env: process.env, print: (line) => console.log(line) });
}
