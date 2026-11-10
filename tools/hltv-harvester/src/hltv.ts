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

/** One line of the match page veto; team is empty for the map left over. */
export type VetoLine = { team: string; action: "ban" | "pick" | "decider"; map: string };

export type MatchPage = {
  teams: { id: number; name: string }[];
  event: string;
  unixMs: number;
  demoPaths: string[];
  maps: string[];
  /** bo1, bo3 or bo5 from the veto box, or empty when the page does not say. */
  format: string;
  veto: VetoLine[];
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
  maps: [...document.querySelectorAll('.mapholder .mapname')].map(m => m.textContent?.trim() ?? ''),
  vetoBoxes: [...document.querySelectorAll('.veto-box')].map(b => {
    const lines = [...b.querySelectorAll('.padding > div')].map(d => d.textContent?.trim() ?? '');
    return lines.length > 0 ? lines : [b.textContent?.trim() ?? ''];
  })
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

const FORMAT_TEXT = /best of (\d)/i;
const VETO_STEP = /^\d+\.\s+(.+?)\s+(removed|picked)\s+(\S+)$/;
const VETO_LEFT = /^\d+\.\s+(\S+)\s+was left over$/;

/** The series format and the ordered veto in the veto box lines, as HLTV writes them. */
export function parseVeto(lines: string[]): { format: string; veto: VetoLine[] } {
  const text = lines.join("\n");
  const n = FORMAT_TEXT.exec(text)?.[1];
  const veto: VetoLine[] = [];
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    const step = VETO_STEP.exec(line);
    if (step) {
      veto.push({ team: step[1] ?? "", action: step[2] === "picked" ? "pick" : "ban", map: step[3] ?? "" });
      continue;
    }
    const left = VETO_LEFT.exec(line);
    if (left) veto.push({ team: "", action: "decider", map: left[1] ?? "" });
  }
  return { format: n ? `bo${n}` : "", veto };
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
  const boxes = Array.isArray(raw.vetoBoxes) ? raw.vetoBoxes : [];
  const { format, veto } = parseVeto(boxes.flatMap((b: unknown) => (Array.isArray(b) ? b.map(str) : [str(b)])));
  return { teams, event: str(raw.event), unixMs: Number.isFinite(unixMs) ? unixMs : 0, demoPaths: [...new Set(demoPaths)], maps, format, veto };
}

export function normalizeName(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]/g, "");
}

// A results row is a candidate when either team name matches a ranked team; the
// match page then confirms it by team id, since names can collide.
export function isCandidate(row: ResultRow, rankedNames: Set<string>): boolean {
  return rankedNames.has(normalizeName(row.team1)) || rankedNames.has(normalizeName(row.team2));
}

export type SelectOptions = { state: HarvestState; rankedNames: Set<string>; rankedOnly: boolean; max: number };

/** What a run visits and why the rest is left: already in the state, or outside the ranking when rankedOnly is set. */
export type Selection = { todo: ResultRow[]; seen: number; unranked: number };

/** Every results row not yet visited (or still without a demo) is a candidate; rankedOnly restores the top-N filter. */
export function selectCandidates(rows: ResultRow[], opts: SelectOptions): Selection {
  const todo: ResultRow[] = [];
  const taken = new Set<number>();
  let seen = 0;
  let unranked = 0;
  for (const row of rows) {
    if (taken.has(row.matchId)) continue;
    if (!needsVisit(opts.state, row.matchId)) {
      seen++;
      continue;
    }
    if (opts.rankedOnly && !isCandidate(row, opts.rankedNames)) {
      unranked++;
      continue;
    }
    taken.add(row.matchId);
    todo.push(row);
  }
  return { todo: todo.slice(0, opts.max), seen, unranked };
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

/**
 * refetch, when set, is a folder of sidecars whose archives are downloaded again instead of walking results.
 * bestMaps picks the best team per map instead, and writes a sidecar for its latest perMap matches on that map.
 */
export type HarvestOptions = {
  days: number; top: number; outDir: string; maxMatches: number; refetch: string;
  bestMaps: boolean; minMaps: number; perMap: number;
  /** When set, the results window is this many hours instead of days: a short run for a frequent schedule. */
  hours: number;
  /** Keep only matches with a team of the top N, as the harvester did before every match with a demo was wanted. */
  rankedOnly: boolean;
  /** A folder of sidecars whose format and veto are read from their match pages, without downloading a demo. */
  vetoBackfill: string;
};

export function parseArgs(argv: string[], defaultOut: string): HarvestOptions {
  const opts: HarvestOptions = { days: 30, top: 50, outDir: defaultOut, maxMatches: 1000, refetch: "", bestMaps: false, minMaps: 10, perMap: 5, vetoBackfill: "", hours: 0, rankedOnly: false };
  for (let i = 0; i < argv.length; i++) {
    // A following flag is never a value, so `--hours --ranked-only` keeps the flag.
    const raw = argv[i + 1] ?? "";
    const next = raw.startsWith("--") ? "" : raw;
    const skip = next === "" ? 0 : 1;
    switch (argv[i]) {
      case "--days": opts.days = positive(next, opts.days); i += skip; break;
      case "--top": opts.top = positive(next, opts.top); i += skip; break;
      case "--max": opts.maxMatches = positive(next, opts.maxMatches); i += skip; break;
      case "--out": if (next !== "") opts.outDir = next; i += skip; break;
      case "--refetch": if (next !== "") opts.refetch = next; i += skip; break;
      case "--best-maps": opts.bestMaps = true; break;
      case "--min-maps": opts.minMaps = positive(next, opts.minMaps); i += skip; break;
      case "--per-map": opts.perMap = positive(next, opts.perMap); i += skip; break;
      case "--hours": opts.hours = positive(next, opts.hours); i += skip; break;
      case "--ranked-only": opts.rankedOnly = true; break;
      case "--veto-backfill": if (next !== "") opts.vetoBackfill = next; i += skip; break;
    }
  }
  return opts;
}

/** The ranking to use: cut at the top N only when the run keeps ranked matches, else whole so every sidecar gets its rank. */
export function cutRanking(ranked: RankedTeam[], opts: { top: number; cut: boolean }): RankedTeam[] {
  return opts.cut ? ranked.filter((t) => t.rank <= opts.top) : ranked;
}

/** The length of the results window in milliseconds: hours when given, else days. */
export function windowMs(opts: Pick<HarvestOptions, "days" | "hours">): number {
  return opts.hours > 0 ? opts.hours * 60 * 60 * 1000 : opts.days * 24 * 60 * 60 * 1000;
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
  format?: string;
  veto?: VetoLine[];
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
    format: match.format || undefined,
    veto: match.veto.length > 0 ? match.veto : undefined,
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
  const veto = (Array.isArray(v.veto) ? v.veto : []).filter(isRecord).flatMap((s): VetoLine[] => {
    const action = s.action;
    return action === "ban" || action === "pick" || action === "decider" ? [{ team: str(s.team), action, map: str(s.map) }] : [];
  });
  const format = str(v.format);
  return {
    source: "hltv", match_id: id, demo_path: path, event: str(v.event), played_at: str(v.played_at), teams, maps,
    format: format || undefined, veto: veto.length > 0 ? veto : undefined,
  };
}

/** The sidecar with the format and veto of its match page, or null when the page holds neither or the sidecar has them. */
export function withSeries(sidecar: Sidecar, match: MatchPage | null): Sidecar | null {
  if ((sidecar.veto?.length ?? 0) > 0 || match === null || (match.format === "" && match.veto.length === 0)) return null;
  return { ...sidecar, format: match.format || sidecar.format, veto: match.veto.length > 0 ? match.veto : sidecar.veto };
}
