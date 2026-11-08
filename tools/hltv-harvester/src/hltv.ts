// Pure HLTV page readers and selection rules for the harvester. The page scripts
// run inside the browser window; everything they return is checked here as unknown.

export const HLTV = "https://www.hltv.org";

export type RankedTeam = { id: number; name: string; rank: number };

export type ResultRow = {
  matchId: number;
  path: string;
  team1: string;
  team2: string;
  event: string;
  unixMs: number;
};

export type MatchPage = {
  teams: { id: number; name: string }[];
  event: string;
  unixMs: number;
  demoPaths: string[];
  maps: string[];
};

export type HarvestRecord = { at: string; status: "downloaded" | "skipped" | "no_demo"; reason?: string; file?: string };

export type HarvestState = { matches: Record<string, HarvestRecord> };

// Page scripts: each one is the body of an expression evaluated in the page.
export const RANKING_SCRIPT = `[...document.querySelectorAll('.ranked-team')].map(t => ({
  pos: t.querySelector('.position')?.textContent?.trim() ?? '',
  name: t.querySelector('.teamLine .name')?.textContent?.trim() ?? '',
  href: t.querySelector('a.moreLink[href^="/team/"], a[href^="/team/"]')?.getAttribute('href') ?? ''
}))`;

export const RESULTS_SCRIPT = `[...document.querySelectorAll('.results-all .result-con')].map(r => ({
  href: r.querySelector('a')?.getAttribute('href') ?? '',
  team1: r.querySelector('.team1 .team')?.textContent?.trim() ?? '',
  team2: r.querySelector('.team2 .team')?.textContent?.trim() ?? '',
  event: r.querySelector('.event-name')?.textContent?.trim() ?? '',
  unix: r.getAttribute('data-zonedgrouping-entry-unix') ?? ''
}))`;

export const MATCH_SCRIPT = `({
  teams: [...document.querySelectorAll('.teamsBox .team a[href^="/team/"]')].map(a => ({ href: a.getAttribute('href') ?? '', name: a.textContent?.trim() ?? '' })),
  event: document.querySelector('.timeAndEvent .event a')?.textContent?.trim() ?? '',
  unix: document.querySelector('.timeAndEvent .time')?.getAttribute('data-unix') ?? '',
  demos: [...document.querySelectorAll('[data-demo-link]')].map(a => a.getAttribute('data-demo-link') ?? ''),
  maps: [...document.querySelectorAll('.mapholder .mapname')].map(m => m.textContent?.trim() ?? '')
})`;

export const CHALLENGE_SCRIPT = `Boolean(document.querySelector('#challenge-form, .cf-turnstile, #cf-wrapper, #challenge-stage')) || /just a moment|un momento/i.test(document.title)`;

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

const TEAM_PATH = /^\/team\/(\d+)\//;
const MATCH_PATH = /^\/matches\/(\d+)\//;
const DEMO_PATH = /^\/download\/demo\/\d+$/;

function teamId(href: string): number {
  const m = TEAM_PATH.exec(href);
  return m ? Number(m[1]) : 0;
}

export function parseRanking(raw: unknown): RankedTeam[] {
  if (!Array.isArray(raw)) return [];
  const out: RankedTeam[] = [];
  for (const item of raw) {
    if (!isRecord(item)) continue;
    const id = teamId(str(item.href));
    const rank = Number(str(item.pos).replace("#", ""));
    const name = str(item.name);
    if (id > 0 && rank > 0 && name !== "") out.push({ id, name, rank });
  }
  return out;
}

export function parseResults(raw: unknown): ResultRow[] {
  if (!Array.isArray(raw)) return [];
  const out: ResultRow[] = [];
  for (const item of raw) {
    if (!isRecord(item)) continue;
    const path = str(item.href);
    const m = MATCH_PATH.exec(path);
    const unixMs = Number(str(item.unix));
    if (!m || !Number.isFinite(unixMs) || unixMs <= 0) continue;
    out.push({ matchId: Number(m[1]), path, team1: str(item.team1), team2: str(item.team2), event: str(item.event), unixMs });
  }
  return out;
}

export function parseMatch(raw: unknown): MatchPage | null {
  if (!isRecord(raw)) return null;
  const teams = (Array.isArray(raw.teams) ? raw.teams : [])
    .filter(isRecord)
    .map((t) => ({ id: teamId(str(t.href)), name: str(t.name) }))
    .filter((t) => t.id > 0);
  const demoPaths = (Array.isArray(raw.demos) ? raw.demos : []).map(str).filter((p) => DEMO_PATH.test(p));
  const maps = (Array.isArray(raw.maps) ? raw.maps : []).map(str).filter((m) => m !== "" && m !== "TBA");
  const unixMs = Number(str(raw.unix));
  return { teams, event: str(raw.event), unixMs: Number.isFinite(unixMs) ? unixMs : 0, demoPaths: [...new Set(demoPaths)], maps };
}

export function normalizeName(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]/g, "");
}

// A results row is a candidate when either team name matches a ranked team; the
// match page then confirms it by team id, since names can collide.
export function isCandidate(row: ResultRow, rankedNames: Set<string>): boolean {
  return rankedNames.has(normalizeName(row.team1)) || rankedNames.has(normalizeName(row.team2));
}

export function rankedTeamIn(match: MatchPage, ranked: RankedTeam[]): RankedTeam | null {
  const byId = new Map(ranked.map((t) => [t.id, t]));
  for (const t of match.teams) {
    const hit = byId.get(t.id);
    if (hit) return hit;
  }
  return null;
}

// A match without a demo yet is retried on a later run; anything else is final.
export function needsVisit(state: HarvestState, matchId: number): boolean {
  const rec = state.matches[String(matchId)];
  return rec === undefined || rec.status === "no_demo";
}

export function parseState(raw: unknown): HarvestState {
  const state: HarvestState = { matches: {} };
  if (!isRecord(raw) || !isRecord(raw.matches)) return state;
  for (const [id, rec] of Object.entries(raw.matches)) {
    if (!isRecord(rec)) continue;
    const status = rec.status;
    if (status === "downloaded" || status === "skipped" || status === "no_demo") {
      state.matches[id] = { at: str(rec.at), status, reason: str(rec.reason) || undefined, file: str(rec.file) || undefined };
    }
  }
  return state;
}

/** refetch, when set, is a folder of sidecars whose archives are downloaded again instead of walking results. */
export type HarvestOptions = { days: number; top: number; outDir: string; maxMatches: number; refetch: string };

export function parseArgs(argv: string[], defaultOut: string): HarvestOptions {
  const opts: HarvestOptions = { days: 30, top: 50, outDir: defaultOut, maxMatches: 1000, refetch: "" };
  for (let i = 0; i < argv.length; i++) {
    const next = argv[i + 1] ?? "";
    switch (argv[i]) {
      case "--days": opts.days = positive(next, opts.days); i++; break;
      case "--top": opts.top = positive(next, opts.top); i++; break;
      case "--max": opts.maxMatches = positive(next, opts.maxMatches); i++; break;
      case "--out": if (next !== "") opts.outDir = next; i++; break;
      case "--refetch": if (next !== "") opts.refetch = next; i++; break;
    }
  }
  return opts;
}

function positive(v: string, fallback: number): number {
  const n = Number(v);
  return Number.isInteger(n) && n > 0 ? n : fallback;
}

// Sidecar written next to each archive so the ingester knows what it holds.
export type Sidecar = {
  source: "hltv";
  match_id: number;
  demo_path: string;
  event: string;
  played_at: string;
  teams: { id: number; name: string; rank: number | null }[];
  maps: string[];
};

export function sidecarFor(matchId: number, demoPath: string, match: MatchPage, ranked: RankedTeam[]): Sidecar {
  const rankOf = new Map(ranked.map((t) => [t.id, t.rank]));
  return {
    source: "hltv",
    match_id: matchId,
    demo_path: demoPath,
    event: match.event,
    played_at: new Date(match.unixMs).toISOString(),
    teams: match.teams.map((t) => ({ id: t.id, name: t.name, rank: rankOf.get(t.id) ?? null })),
    maps: match.maps,
  };
}

/** A sidecar read back from disk, or null when it is not an HLTV demo download of a real match. */
export function parseSidecar(v: unknown): Sidecar | null {
  if (!isRecord(v) || v.source !== "hltv") return null;
  const id = v.match_id;
  const path = str(v.demo_path);
  if (typeof id !== "number" || !Number.isInteger(id) || id <= 0 || !DEMO_PATH.test(path)) return null;
  const teams = (Array.isArray(v.teams) ? v.teams : []).filter(isRecord).map((t) => ({
    id: typeof t.id === "number" ? t.id : 0,
    name: str(t.name),
    rank: typeof t.rank === "number" ? t.rank : null,
  }));
  const maps = (Array.isArray(v.maps) ? v.maps : []).map(str).filter((m) => m !== "");
  return { source: "hltv", match_id: id, demo_path: path, event: str(v.event), played_at: str(v.played_at), teams, maps };
}
