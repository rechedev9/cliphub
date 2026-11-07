// HLTV demo harvester: a visible browser window that walks HLTV results the way a
// person would, keeps matches with a ranked team and downloads their demo archives
// one at a time. It never solves a Cloudflare challenge: it waits for the user.
//
//   pnpm start -- --days 30 --top 50 --out C:\Users\me\cliphub-hltv
import { app, BrowserWindow, session, type DownloadItem, type Session } from "electron";
import { appendFileSync, existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import {
  CHALLENGE_SCRIPT, HLTV, MATCH_SCRIPT, RANKING_SCRIPT, RESULTS_SCRIPT,
  isCandidate, needsVisit, normalizeName, parseArgs, parseMatch, parseRanking, parseResults, parseState,
  rankedTeamIn, sidecarFor, type HarvestRecord, type HarvestState, type RankedTeam, type ResultRow,
} from "./hltv.ts";

const PAGE_DELAY_MS: [number, number] = [6000, 12000];
const CHALLENGE_WAIT_MS = 30 * 60 * 1000;
const DOWNLOAD_TIMEOUT_MS = 60 * 60 * 1000;
const RESULTS_PAGE_SIZE = 100;

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
    await this.win.loadURL(url);
    if (!(await this.challenged())) return;
    log("class=challenge message=\"Cloudflare pide verificación: complétala en la ventana del recolector\"");
    this.win.show();
    this.win.focus();
    const deadline = Date.now() + CHALLENGE_WAIT_MS;
    while (Date.now() < deadline) {
      await sleep(5000);
      if (!(await this.challenged())) {
        log("class=challenge_cleared");
        await this.win.loadURL(url);
        return;
      }
    }
    throw new Error("Cloudflare challenge was not completed within 30 minutes");
  }

  private async challenged(): Promise<boolean> {
    const v: unknown = await this.win.webContents.executeJavaScript(CHALLENGE_SCRIPT);
    return v === true;
  }

  async read(script: string): Promise<unknown> {
    const v: unknown = await this.win.webContents.executeJavaScript(script);
    return v;
  }

  download(ses: Session, url: string, target: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("download timed out")), DOWNLOAD_TIMEOUT_MS);
      ses.once("will-download", (_event, item: DownloadItem) => {
        const part = `${target}.part`;
        item.setSavePath(part);
        item.once("done", (_e, state) => {
          clearTimeout(timer);
          if (state !== "completed") {
            reject(new Error(`download ${state}`));
            return;
          }
          renameSync(part, target);
          resolve();
        });
      });
      this.win.webContents.downloadURL(url);
    });
  }
}

async function rankedTeams(browser: Browser): Promise<RankedTeam[]> {
  await browser.open(`${HLTV}/ranking/teams`);
  const ranked = parseRanking(await browser.read(RANKING_SCRIPT)).filter((t) => t.rank <= opts.top);
  if (ranked.length < Math.min(opts.top, 10)) throw new Error(`ranking page gave ${ranked.length} teams`);
  return ranked;
}

async function recentResults(browser: Browser, cutoffMs: number): Promise<ResultRow[]> {
  const rows: ResultRow[] = [];
  for (let offset = 0; offset < 5000; offset += RESULTS_PAGE_SIZE) {
    await pause();
    await browser.open(`${HLTV}/results?offset=${offset}`);
    const page = parseResults(await browser.read(RESULTS_SCRIPT));
    if (page.length === 0) break;
    rows.push(...page.filter((r) => r.unixMs >= cutoffMs));
    if (page.every((r) => r.unixMs < cutoffMs)) break;
    if (page[page.length - 1].unixMs < cutoffMs) break;
  }
  return rows;
}

async function harvest(win: BrowserWindow, ses: Session): Promise<void> {
  mkdirSync(downloadsDir, { recursive: true });
  const state = loadState();
  const browser = new Browser(win);
  const ranked = await rankedTeams(browser);
  const names = new Set(ranked.map((t) => normalizeName(t.name)));
  const cutoff = Date.now() - opts.days * 24 * 60 * 60 * 1000;
  const rows = await recentResults(browser, cutoff);
  const todo = rows.filter((r) => isCandidate(r, names) && needsVisit(state, r.matchId)).slice(0, opts.maxMatches);
  log(`class=listed results=${rows.length} candidates=${todo.length} ranked=${ranked.length} days=${opts.days}`);
  let downloaded = 0;
  for (const row of todo) {
    await pause();
    await browser.open(`${HLTV}${row.path}`);
    const match = parseMatch(await browser.read(MATCH_SCRIPT));
    if (match === null) {
      record(state, row.matchId, { status: "skipped", reason: "unreadable match page" });
      continue;
    }
    if (rankedTeamIn(match, ranked) === null) {
      record(state, row.matchId, { status: "skipped", reason: "no ranked team" });
      continue;
    }
    const demoPath = match.demoPaths[0];
    if (demoPath === undefined) {
      record(state, row.matchId, { status: "no_demo" });
      continue;
    }
    if (match.unixMs === 0) match.unixMs = row.unixMs;
    const file = join(downloadsDir, `${row.matchId}.rar`);
    log(`class=downloading match=${row.matchId} event="${match.event}" maps=${match.maps.length}`);
    try {
      await browser.download(ses, `${HLTV}${demoPath}`, file);
    } catch (err) {
      log(`class=download_failed match=${row.matchId} error="${String(err)}"`);
      continue;
    }
    writeFileSync(join(downloadsDir, `${row.matchId}.json`), JSON.stringify(sidecarFor(row.matchId, demoPath, match, ranked), null, 2));
    record(state, row.matchId, { status: "downloaded", file });
    downloaded++;
  }
  log(`class=done downloaded=${downloaded}`);
}

app.whenReady().then(async () => {
  const ses = session.fromPartition("persist:hltv");
  const win = new BrowserWindow({ width: 1280, height: 860, show: true, title: "ClipHub: recolector HLTV", webPreferences: { session: ses, contextIsolation: true, sandbox: true } });
  try {
    await harvest(win, ses);
  } catch (err) {
    log(`class=fatal error="${String(err)}"`);
    process.exitCode = 1;
  }
  app.quit();
});
