// HLTV demo harvester: a visible browser window that walks HLTV results the way a
// person would and downloads the demo archive of every match that has one, one at a
// time. It never solves a Cloudflare challenge: it waits for the user.
// --ranked-only keeps only matches with a top-N team. --hours N reads only the last N hours,
// for a frequent run: keep the nightly --days 7 run beside it, because a results page that
// fails mid-walk cuts a window short (the log says results_incomplete=true) and the longer
// nightly window picks up what a short run missed.
//
//   pnpm start -- --days 30 --top 50 --out C:\Users\me\cliphub-hltv
//   pnpm start -- --best-maps --days 90 --top 20 --min-maps 10 --per-map 5
//   pnpm start -- --veto-backfill C:\Users\me\cliphub-hltv\series-sidecars
import { app, BrowserWindow, session, type DownloadItem, type Session } from "electron";
import { appendFileSync, existsSync, mkdirSync, readdirSync, readFileSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import {
  CHALLENGE_SCRIPT, HLTV, MATCH_SCRIPT, RANKING_SCRIPT, RESULTS_SCRIPT,
  cutRanking, normalizeName, parseArgs, parseMatch, parseRanking, parseResults, parseSidecar, parseState,
  rankedTeamIn, selectCandidates, sidecarFor, windowMs, withSeries, type HarvestRecord, type HarvestState, type RankedTeam, type ResultRow,
} from "./hltv.ts";
import { ChallengeTimeoutError, FailureBreaker, isTransient, withRetry } from "./retry.ts";
import { MAP_STATS_SCRIPT, isoDay, parseTeamMapStats, pickBestPerMap, teamSlug, type BestMapPick, type TeamStats } from "./best-maps.ts";
import {
  demoFileName, faceitSidecarFor, isSidecarOf, matchScript, parseFaceitMatch, parseReply, parseSignedUrl, replyKind, roomIds, roomUrl, sidecarName, signScript,
  type ApiReply, type FaceitMatch,
} from "./faceit.ts";

const PAGE_DELAY_MS: [number, number] = [6000, 12000];
const CHALLENGE_WAIT_MS = 30 * 60 * 1000;
const DOWNLOAD_TIMEOUT_MS = 60 * 60 * 1000;
const RESULTS_PAGE_SIZE = 100;
const LOAD_ATTEMPTS = 4;
const LOAD_BACKOFF_MS = 15_000;
const MAX_CONSECUTIVE_FAILURES = 3;
const SIGN_IN_POLL_MS = 15_000;
const FACEIT_REQUEST_MS = 60_000;

const opts = parseArgs(process.argv.slice(2), join(homedir(), "cliphub-hltv"));
const downloadsDir = join(opts.outDir, "downloads");
const statePath = join(opts.outDir, "harvest-state.json");

// Electron on Windows drops console output when it is redirected, so the log is a file.
function log(msg: string): void {
  const line = `${new Date().toISOString()} harvester ${msg}`;
  console.log(line);
  appendFileSync(join(opts.outDir, "harvester.log"), `${line}\n`);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function pause(): Promise<void> {
  const [min, max] = PAGE_DELAY_MS;
  return sleep(min + Math.random() * (max - min));
}

function loadState(): HarvestState {
  if (!existsSync(statePath)) return { matches: {} };
  return parseState(JSON.parse(readFileSync(statePath, "utf8")));
}

function record(state: HarvestState, matchId: number, rec: Omit<HarvestRecord, "at">): void {
  state.matches[String(matchId)] = { at: new Date().toISOString(), ...rec };
  writeFileSync(statePath, JSON.stringify(state, null, 2));
}

class Browser {
  constructor(private readonly win: BrowserWindow) {}

  // open loads a page and, when Cloudflare shows a challenge, hands the window
  // to the user and waits for the page to come back.
  async open(url: string): Promise<void> {
    await this.load(url);
    if (!(await this.challenged())) return;
    log("class=challenge message=\"Cloudflare pide verificación: complétala en la ventana del recolector\"");
    this.reveal();
    const deadline = Date.now() + CHALLENGE_WAIT_MS;
    while (Date.now() < deadline) {
      await sleep(5000);
      if (!(await this.challenged())) {
        log("class=challenge_cleared");
        await this.load(url);
        return;
      }
    }
    throw new ChallengeTimeoutError();
  }

  // load retries a cut connection (ERR_CONNECTION_CLOSED and friends) with a growing wait.
  private load(url: string): Promise<void> {
    return withRetry(() => this.win.loadURL(url), {
      attempts: LOAD_ATTEMPTS,
      baseMs: LOAD_BACKOFF_MS,
      sleep,
      isTransient,
      onRetry: (attempt, err) => log(`class=load_retry attempt=${attempt} url="${url}" error="${String(err)}"`),
    });
  }

  reveal(): void {
    this.win.show();
    this.win.focus();
  }

  private async challenged(): Promise<boolean> {
    const v: unknown = await this.win.webContents.executeJavaScript(CHALLENGE_SCRIPT);
    return v === true;
  }

  async read(script: string): Promise<unknown> {
    const v: unknown = await this.win.webContents.executeJavaScript(script);
    return v;
  }

  readWithin(script: string, ms: number): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("page script timed out")), ms);
      this.read(script).then(resolve, reject).finally(() => clearTimeout(timer));
    });
  }

  download(ses: Session, url: string, target: string): Promise<void> {
    const part = `${target}.part`;
    const wanted = new URL(url).href;
    return new Promise((resolve, reject) => {
      let item: DownloadItem | null = null;
      let settled = false;
      const dropPart = (): void => {
        try {
          rmSync(part, { force: true });
        } catch (err) {
          log(`class=part_left file="${part}" error="${String(err)}"`);
        }
      };
      const finish = (err: Error | null): void => {
        if (settled) {
          if (err !== null) dropPart();
          return;
        }
        settled = true;
        clearTimeout(timer);
        ses.removeListener("will-download", onStart);
        if (err === null) {
          resolve();
          return;
        }
        dropPart();
        reject(err);
      };
      const onStart = (_event: Electron.Event, started: DownloadItem): void => {
        if (item !== null || started.getURLChain()[0] !== wanted) {
          started.cancel();
          return;
        }
        item = started;
        started.setSavePath(part);
        started.once("done", (_e, state) => {
          if (state !== "completed") {
            finish(new Error(`download ${state}`));
            return;
          }
          try {
            renameSync(part, target);
            finish(null);
          } catch (err) {
            finish(err instanceof Error ? err : new Error(String(err)));
          }
        });
      };
      const timer = setTimeout(() => {
        item?.cancel();
        finish(new Error("download timed out"));
      }, DOWNLOAD_TIMEOUT_MS);
      ses.on("will-download", onStart);
      this.win.webContents.downloadURL(url);
    });
  }
}

// Without --ranked-only the ranking only fills each sidecar's rank, so a broken page must not stop the run.
async function rankedTeams(browser: Browser): Promise<RankedTeam[]> {
  const cut = opts.rankedOnly || opts.bestMaps;
  if (cut) return readRanking(browser, cut);
  try {
    return await readRanking(browser, cut);
  } catch (err) {
    if (err instanceof ChallengeTimeoutError) throw err;
    log(`class=ranking_failed error="${String(err)}"`);
    return [];
  }
}

async function readRanking(browser: Browser, cut: boolean): Promise<RankedTeam[]> {
  await browser.open(`${HLTV}/ranking/teams`);
  const ranked = cutRanking(parseRanking(await browser.read(RANKING_SCRIPT)), { top: opts.top, cut });
  if (ranked.length < Math.min(opts.top, 10)) throw new Error(`ranking page gave ${ranked.length} teams`);
  return ranked;
}

type Listing = { rows: ResultRow[]; complete: boolean };

// complete is false when a results page failed after the first, so the window was cut short.
async function recentResults(browser: Browser, cutoffMs: number): Promise<Listing> {
  const rows: ResultRow[] = [];
  let complete = true;
  for (let offset = 0; offset < 5000; offset += RESULTS_PAGE_SIZE) {
    await pause();
    let page: ResultRow[];
    try {
      await browser.open(`${HLTV}/results?offset=${offset}`);
      page = parseResults(await browser.read(RESULTS_SCRIPT));
    } catch (err) {
      if (rows.length === 0 || err instanceof ChallengeTimeoutError) throw err;
      log(`class=results_page_failed offset=${offset} error="${String(err)}"`);
      complete = false;
      break;
    }
    if (page.length === 0) break;
    rows.push(...page.filter((r) => r.unixMs >= cutoffMs));
    if (page.every((r) => r.unixMs < cutoffMs)) break;
    if (page[page.length - 1].unixMs < cutoffMs) break;
  }
  return { rows, complete };
}

async function harvest(win: BrowserWindow, ses: Session): Promise<void> {
  mkdirSync(downloadsDir, { recursive: true });
  const state = loadState();
  const browser = new Browser(win);
  const ranked = await rankedTeams(browser);
  const names = new Set(ranked.map((t) => normalizeName(t.name)));
  const { rows, complete } = await recentResults(browser, Date.now() - windowMs(opts));
  const { todo, seen, unranked } = selectCandidates(rows, { state, rankedNames: names, rankedOnly: opts.rankedOnly, max: opts.maxMatches });
  log(`class=listed results=${rows.length} candidates=${todo.length} already_seen=${seen} unranked=${unranked} ranked=${ranked.length} ranked_only=${opts.rankedOnly} days=${opts.days} hours=${opts.hours} results_incomplete=${!complete}`);
  const breaker = new FailureBreaker(MAX_CONSECUTIVE_FAILURES);
  let downloaded = 0;
  let failed = 0;
  for (const row of todo) {
    try {
      if (await visit({ browser, ses, state, ranked, row })) downloaded++;
      breaker.success();
    } catch (err) {
      if (err instanceof ChallengeTimeoutError) throw err;
      failed++;
      log(`class=match_failed match=${row.matchId} error="${String(err)}"`);
      if (breaker.failure()) throw new Error(`${MAX_CONSECUTIVE_FAILURES} matches failed in a row: offline or blocked`);
    }
  }
  log(`class=done downloaded=${downloaded} failed=${failed} results_incomplete=${!complete}`);
}

type VisitContext = { browser: Browser; ses: Session; state: HarvestState; ranked: RankedTeam[]; row: ResultRow };

// visit reads one match page and downloads its demo archive; it returns true when an archive was saved.
async function visit(ctx: VisitContext): Promise<boolean> {
  const { browser, ses, state, ranked, row } = ctx;
  await pause();
  await browser.open(`${HLTV}${row.path}`);
  const match = parseMatch(await browser.read(MATCH_SCRIPT));
  if (match === null) {
    record(state, row.matchId, { status: "skipped", reason: "unreadable match page" });
    return false;
  }
  if (opts.rankedOnly && rankedTeamIn(match, ranked) === null) {
    record(state, row.matchId, { status: "skipped", reason: "no ranked team" });
    return false;
  }
  const demoPath = match.demoPaths[0];
  if (demoPath === undefined) {
    record(state, row.matchId, { status: "no_demo" });
    return false;
  }
  if (match.unixMs === 0) match.unixMs = row.unixMs;
  const file = join(downloadsDir, `${row.matchId}.rar`);
  log(`class=downloading match=${row.matchId} event="${match.event}" maps=${match.maps.length}`);
  try {
    await browser.download(ses, `${HLTV}${demoPath}`, file);
  } catch (err) {
    log(`class=download_failed match=${row.matchId} error="${String(err)}"`);
    return false;
  }
  writeFileSync(join(downloadsDir, `${row.matchId}.json`), JSON.stringify(sidecarFor(row.matchId, demoPath, match, ranked), null, 2));
  record(state, row.matchId, { status: "downloaded", file });
  return true;
}

// refetch downloads again the archives of sidecars kept from earlier runs, so their demos can be recorded.
async function refetch(win: BrowserWindow, ses: Session): Promise<void> {
  mkdirSync(downloadsDir, { recursive: true });
  const browser = new Browser(win);
  await browser.open(`${HLTV}/results`);
  let downloaded = 0;
  for (const name of readdirSync(opts.refetch).filter((f) => f.endsWith(".json")).sort()) {
    if (downloaded === opts.maxMatches) break;
    const sidecar = parseSidecar(JSON.parse(readFileSync(join(opts.refetch, name), "utf8")));
    if (sidecar === null) {
      log(`class=refetch_skipped file=${name}`);
      continue;
    }
    const file = join(downloadsDir, `${sidecar.match_id}.rar`);
    if (existsSync(file)) continue;
    await pause();
    log(`class=refetching match=${sidecar.match_id}`);
    try {
      await browser.download(ses, `${HLTV}${sidecar.demo_path}`, file);
    } catch (err) {
      log(`class=download_failed match=${sidecar.match_id} error="${String(err)}"`);
      continue;
    }
    writeFileSync(join(downloadsDir, `${sidecar.match_id}.json`), JSON.stringify(sidecar, null, 2));
    downloaded++;
  }
  log(`class=done refetched=${downloaded}`);
}

// vetoBackfill reads the format and veto of each sidecar's match page into the sidecar; it downloads no demo.
async function vetoBackfill(win: BrowserWindow): Promise<void> {
  const browser = new Browser(win);
  let filled = 0;
  for (const name of readdirSync(opts.vetoBackfill).filter((f) => f.endsWith(".json")).sort()) {
    if (filled === opts.maxMatches) break;
    const path = join(opts.vetoBackfill, name);
    const sidecar = parseSidecar(JSON.parse(readFileSync(path, "utf8")));
    if (sidecar === null || (sidecar.veto?.length ?? 0) > 0) continue;
    await pause();
    await browser.open(`${HLTV}/matches/${sidecar.match_id}/match`);
    const next = withSeries(sidecar, parseMatch(await browser.read(MATCH_SCRIPT)));
    if (next === null) {
      log(`class=veto_missing match=${sidecar.match_id}`);
      continue;
    }
    writeFileSync(path, JSON.stringify(next, null, 2));
    log(`class=veto_filled match=${sidecar.match_id} format=${next.format ?? ""} steps=${next.veto?.length ?? 0}`);
    filled++;
  }
  log(`class=done veto_filled=${filled}`);
}

// bestMaps reads every ranked team's map stats, picks the best team per map and writes a
// sidecar for its latest matches on that map into refetch-best, for the clip backfill.
async function bestMaps(win: BrowserWindow): Promise<void> {
  const browser = new Browser(win);
  const ranked = await rankedTeams(browser);
  const end = Date.now();
  const start = end - opts.days * 24 * 60 * 60 * 1000;
  const range = `startDate=${isoDay(start)}&endDate=${isoDay(end)}`;
  const teams: TeamStats[] = [];
  for (const t of ranked) {
    await pause();
    await browser.open(`${HLTV}/stats/teams/maps/${t.id}/${teamSlug(t.name)}?${range}`);
    const records = parseTeamMapStats(await browser.read(MAP_STATS_SCRIPT));
    log(`class=team_maps team="${t.name}" maps=${records.length}`);
    teams.push({ ...t, records });
  }
  const picks = pickBestPerMap(teams, opts.minMaps);
  if (picks.length === 0) throw new Error("no team map stats read: the HLTV stats page layout may have changed");
  writeFileSync(join(opts.outDir, "best-maps.json"), JSON.stringify(picks, null, 2));
  const sidecars = join(opts.outDir, "refetch-best");
  mkdirSync(sidecars, { recursive: true });
  for (const pick of picks) {
    log(`class=best_map map=${pick.map} team="${pick.team}" maps=${pick.maps} win_rate=${pick.win_rate}`);
    await pickMatches(browser, pick, { range, start, ranked, sidecars });
  }
  log(`class=done picks=${picks.length}`);
}

type PickContext = { range: string; start: number; ranked: RankedTeam[]; sidecars: string };

async function pickMatches(browser: Browser, pick: BestMapPick, ctx: PickContext): Promise<void> {
  await pause();
  await browser.open(`${HLTV}/results?team=${pick.team_id}&map=${pick.map}&${ctx.range}`);
  const rows = parseResults(await browser.read(RESULTS_SCRIPT)).filter((r) => r.unixMs >= ctx.start).slice(0, opts.perMap);
  for (const row of rows) {
    const out = join(ctx.sidecars, `${row.matchId}.json`);
    if (existsSync(out)) continue;
    await pause();
    await browser.open(`${HLTV}${row.path}`);
    const match = parseMatch(await browser.read(MATCH_SCRIPT));
    const demoPath = match?.demoPaths[0];
    if (match === null || demoPath === undefined) {
      log(`class=best_no_demo match=${row.matchId}`);
      continue;
    }
    if (match.unixMs === 0) match.unixMs = row.unixMs;
    writeFileSync(out, JSON.stringify(sidecarFor(row.matchId, demoPath, match, ctx.ranked), null, 2));
  }
  log(`class=best_matches map=${pick.map} team="${pick.team}" matches=${rows.length}`);
}

type FaceitRun = { browser: Browser; ses: Session; dir: string; downloaded: number };

async function faceit(win: BrowserWindow, ses: Session): Promise<void> {
  const dir = join(opts.outDir, "faceit");
  mkdirSync(dir, { recursive: true });
  const rooms = roomIds(existsSync(opts.faceit) ? readFileSync(opts.faceit, "utf8") : opts.faceit);
  if (rooms.length === 0) throw new Error(`no FACEIT room id in "${opts.faceit}"`);
  const run: FaceitRun = { browser: new Browser(win), ses, dir, downloaded: 0 };
  const breaker = new FailureBreaker(MAX_CONSECUTIVE_FAILURES);
  let failed = 0;
  try {
    for (const roomId of rooms) {
      if (run.downloaded >= opts.maxMatches) break;
      try {
        await faceitRoom(run, roomId);
        breaker.success();
      } catch (err) {
        if (err instanceof ChallengeTimeoutError) throw err;
        failed++;
        log(`class=faceit_failed room=${roomId} error="${String(err)}"`);
        if (breaker.failure()) throw new Error(`${MAX_CONSECUTIVE_FAILURES} FACEIT rooms failed in a row: offline or blocked`);
      }
    }
  } finally {
    log(`class=done faceit_rooms=${rooms.length} downloaded=${run.downloaded} failed=${failed}`);
  }
  if (failed > 0) process.exitCode = 1;
}

async function faceitRoom(run: FaceitRun, roomId: string): Promise<void> {
  await pause();
  await run.browser.open(roomUrl(roomId));
  const reply = await faceitCall(run.browser, (ms) => matchScript(roomId, ms));
  if (reply === null) throw new Error("FACEIT has no such room");
  const match = parseFaceitMatch(reply.body, roomId);
  if (match === null) throw new Error("FACEIT match unreadable or not finished");
  if (match.demoUrls.length === 0) {
    log(`class=faceit_no_demo room=${roomId}`);
    return;
  }
  for (const resource of match.demoUrls) {
    if (run.downloaded >= opts.maxMatches) return;
    await faceitDemo(run, match, resource);
  }
}

async function faceitDemo(run: FaceitRun, match: FaceitMatch, resource: string): Promise<void> {
  const name = demoFileName(resource);
  const file = join(run.dir, name);
  const sidecar = join(run.dir, sidecarName(name));
  if (existsSync(file)) {
    if (!existsSync(sidecar) || !isSidecarOf(readFileSync(sidecar, "utf8"), name)) writeSidecar(sidecar, match, resource);
    return;
  }
  const reply = await faceitCall(run.browser, (ms) => signScript(resource, ms));
  if (reply === null) {
    log(`class=faceit_demo_gone room=${match.roomId} file=${name}`);
    return;
  }
  const url = parseSignedUrl(reply.body);
  if (url === "") throw new Error("FACEIT gave no signed demo link");
  log(`class=downloading room=${match.roomId} file=${name} map=${match.map}`);
  await run.browser.download(run.ses, url, file);
  run.downloaded++;
  writeSidecar(sidecar, match, resource);
}

function writeSidecar(path: string, match: FaceitMatch, resource: string): void {
  const tmp = `${path}.tmp`;
  writeFileSync(tmp, JSON.stringify(faceitSidecarFor(match, resource), null, 2));
  renameSync(tmp, path);
}

async function faceitCall(browser: Browser, script: (ms: number) => string): Promise<ApiReply | null> {
  const deadline = Date.now() + CHALLENGE_WAIT_MS;
  let waiting = false;
  for (;;) {
    const ms = Math.max(1000, Math.min(FACEIT_REQUEST_MS, deadline - Date.now()));
    const reply = parseReply(await browser.readWithin(script(ms), ms + 5000).catch(() => null));
    const kind = replyKind(reply, waiting);
    if (kind === "ok") return reply;
    if (kind === "gone") return null;
    if (kind === "error") throw new Error(`FACEIT answered ${reply.status}`);
    if (Date.now() >= deadline) throw new ChallengeTimeoutError();
    if (!waiting) {
      log("class=faceit_needs_user message=\"FACEIT pide iniciar sesión o verificación: hazlo en la ventana del recolector\"");
      browser.reveal();
      waiting = true;
    }
    await sleep(Math.min(SIGN_IN_POLL_MS, Math.max(0, deadline - Date.now())));
  }
}

app.whenReady().then(async () => {
  if (opts.invalid !== "") {
    mkdirSync(opts.outDir, { recursive: true });
    log(`class=fatal error="${opts.invalid}"`);
    app.exit(2);
    return;
  }
  const ses = session.fromPartition("persist:hltv");
  const win = new BrowserWindow({ width: 1280, height: 860, show: true, title: "ClipHub: recolector HLTV", webPreferences: { session: ses, contextIsolation: true, sandbox: true } });
  try {
    if (opts.faceit !== "") await faceit(win, ses);
    else if (opts.vetoBackfill !== "") await vetoBackfill(win);
    else if (opts.bestMaps) await bestMaps(win);
    else await (opts.refetch === "" ? harvest(win, ses) : refetch(win, ses));
  } catch (err) {
    log(`class=fatal error="${String(err)}"`);
    process.exitCode = 1;
  }
  app.exit(Number(process.exitCode ?? 0));
});
