#!/usr/bin/env node
// Verify a change in the real ClipHub Studio desktop app (Electron), in the
// background, against a disposable copy of the user's Studio data.
//
//   node .claude/skills/verify-desktop/verify-desktop.mjs [prepare|run|all] [options]
//
// prepare  builds desktop/dist, the Go binaries and the Next standalone server
//          of this checkout into desktop/build-resources (no HLAE download).
// run      boots Electron on the isolated profile, walks every screen at each
//          width and runs an optional change-specific --check script.
// all      prepare + run (default).
//
// Options:
//   --skip-web --skip-go --skip-desktop   reuse that part of the last prepare
//   --fresh-data         recopy the profile's data from the real Studio
//   --widths 1440,1024,760               window content widths (default)
//   --routes /clips,/players             replace the default route pass
//   --no-pass                            skip the route pass (only --check)
//   --check <file.mjs>   default export async (ctx) => {...}; see SKILL.md
//   --allow-writes       let non-GET API calls through (writes go to the copy,
//                        but a capture could launch CS2 - avoid unless needed)
//   --keep-open          leave the app running after the run (Ctrl+C to stop)
import { spawnSync } from 'node:child_process';
import {
  cpSync, existsSync, linkSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync,
} from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const repo = spawnSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).stdout.trim();
if (!repo) throw new Error('run from inside a ClipHub checkout');
const desktop = path.join(repo, 'desktop');
const web = path.join(repo, 'web');
const resources = path.join(desktop, 'build-resources');
const HOME = process.env.CLIPHUB_VERIFY_HOME ?? 'C:\\temp\\cliphub-verify';
const profile = path.join(HOME, 'profile');
const realStudio = path.join(process.env.APPDATA ?? '', 'cliphub-studio');
const BIG_FILE = 100 * 1024 * 1024;
// --fresh-data deletes <HOME>/profile/data: HOME must never overlap the real Studio.
if (process.env.APPDATA && path.resolve(HOME).toLowerCase().startsWith(path.resolve(process.env.APPDATA).toLowerCase())) {
  throw new Error(`CLIPHUB_VERIFY_HOME must not live under %APPDATA% (${HOME})`);
}

// Drops matching variables; Windows environment names are case-insensitive.
function without(environment, pattern) {
  return Object.fromEntries(Object.entries(environment).filter(([name]) => !pattern.test(name)));
}

const argv = process.argv.slice(2);
const mode = ['prepare', 'run', 'all'].includes(argv[0]) ? argv.shift() : 'all';
const flag = (name) => argv.includes(`--${name}`);
const option = (name, fallback) => {
  const index = argv.indexOf(`--${name}`);
  return index >= 0 && argv[index + 1] !== undefined ? argv[index + 1] : fallback;
};

function step(label, fn) {
  const start = Date.now();
  process.stdout.write(`[verify-desktop] ${label}...`);
  const result = fn();
  process.stdout.write(` ${((Date.now() - start) / 1000).toFixed(1)}s\n`);
  return result;
}

const redact = (text) => {
  const key = (process.env.FACEIT_API_KEY ?? '').trim();
  return key ? String(text).replaceAll(key, '<FACEIT_API_KEY>') : String(text);
};

// Only pnpm (a .cmd shim) needs a shell on Windows; go and robocopy get argv as is.
function sh(command, args, options = {}) {
  const viaShell = process.platform === 'win32' && command === 'pnpm';
  const result = viaShell
    ? spawnSync([command, ...args].join(' '), { cwd: repo, encoding: 'utf8', shell: true, maxBuffer: 64 << 20, ...options })
    : spawnSync(command, args, { cwd: repo, encoding: 'utf8', maxBuffer: 64 << 20, ...options });
  const ok = options.okCodes ? options.okCodes.includes(result.status) : result.status === 0;
  if (!ok) {
    // Redact before slicing so a key cut by the slice boundary is never printed.
    process.stdout.write('\n' + redact(result.stdout ?? '').slice(-4000) + redact(result.stderr ?? '').slice(-4000));
    throw new Error(redact(`${command} ${args.join(' ')} exited ${result.status ?? result.error}`));
  }
  return result.stdout;
}

// A previous run's app keeps build-resources/web/server.js open (EBUSY on rebuild).
// The PID file can be stale (run killed before cleanup) and Windows reuses PIDs,
// so kill the tree only while that PID still belongs to an electron.exe.
function stopPreviousApp() {
  const pidFile = path.join(HOME, 'app.pid');
  if (!existsSync(pidFile)) return;
  const pid = readFileSync(pidFile, 'utf8').trim();
  if (/^\d+$/.test(pid)) {
    const list = spawnSync('tasklist', ['/FI', `PID eq ${pid}`, '/FI', 'IMAGENAME eq electron.exe', '/NH'], { encoding: 'utf8' });
    if (/electron\.exe/i.test(list.stdout ?? '')) spawnSync('taskkill', ['/PID', pid, '/T', '/F'], { encoding: 'utf8' });
  }
  rmSync(pidFile, { force: true });
}

// Exactly the binaries desktop/scripts/assemble.mjs ships in the installer, so
// sibling detection (e.g. zv-composer) behaves as in the user's Studio.
const GO_COMMANDS = ['zv-orchestrator', 'zv-editor', 'zv-recorder'];

function prepare() {
  stopPreviousApp();
  if (!flag('skip-desktop')) {
    if (!existsSync(path.join(desktop, 'node_modules'))) step('desktop install', () => sh('pnpm', ['--dir', 'desktop', 'install', '--frozen-lockfile']));
    if (!existsSync(path.join(desktop, 'node_modules', 'electron', 'dist'))) step('electron binary', () => sh('node', ['node_modules/electron/install.js'], { cwd: desktop }));
    step('desktop dist', () => sh('pnpm', ['--dir', 'desktop', 'run', 'build']));
  }
  if (!flag('skip-go')) {
    // Start clean so binaries from an older, wider build do not linger as siblings.
    rmSync(path.join(resources, 'bin'), { recursive: true, force: true });
    mkdirSync(path.join(resources, 'bin'), { recursive: true });
    // Same key embedding as scripts/build.ps1, so Jugadores works as in the installer.
    const key = (process.env.FACEIT_API_KEY ?? '').trim();
    const ldflags = key && /^[A-Za-z0-9._-]+$/.test(key) ? ['-ldflags', `-X main.embeddedFaceitAPIKey=${key}`] : [];
    step('go binaries', () => {
      sh('go', ['build', '-o', path.join(resources, 'bin') + path.sep, ...GO_COMMANDS.filter((c) => c !== 'zv-orchestrator').map((c) => `./cmd/${c}`)]);
      sh('go', ['build', ...ldflags, '-o', path.join(resources, 'bin', 'zv-orchestrator.exe'), './cmd/zv-orchestrator']);
    });
  }
  if (!flag('skip-web')) {
    if (!existsSync(path.join(web, 'node_modules'))) step('web install', () => sh('pnpm', ['--dir', 'web', 'install', '--frozen-lockfile']));
    // Like assemble.mjs: the xAI key never reaches the bundled web build.
    step('web build', () => sh('pnpm', ['--dir', 'web', 'run', 'build'], { env: without(process.env, /^xai_api_key$/i) }));
    step('stage web', () => {
      const out = path.join(resources, 'web');
      rmSync(out, { recursive: true, force: true });
      cpSync(path.join(web, '.next', 'standalone'), out, { recursive: true });
      cpSync(path.join(web, '.next', 'static'), path.join(out, '.next', 'static'), { recursive: true });
      if (existsSync(path.join(web, 'public'))) cpSync(path.join(web, 'public'), path.join(out, 'public'), { recursive: true });
    });
  }
  // Music as scripts/assemble.mjs stages it. HLAE is not staged: the profile
  // carries the installed, hash-verified tools of the real Studio instead.
  const musicSrc = path.join(repo, 'data', 'music');
  const musicOut = path.join(resources, 'music');
  rmSync(musicOut, { recursive: true, force: true });
  mkdirSync(musicOut, { recursive: true });
  cpSync(path.join(musicSrc, 'catalog.json'), path.join(musicOut, 'catalog.json'));
  const catalog = JSON.parse(readFileSync(path.join(musicSrc, 'catalog.json'), 'utf8'));
  const mainCheckout = path.dirname(path.resolve(repo, sh('git', ['rev-parse', '--git-common-dir']).trim()));
  const missing = [];
  for (const t of catalog.tracks ?? []) {
    if (t.downloadUrl || !t.id || !t.ext) continue;
    const file = path.join(musicSrc, `${t.id}.${t.ext}`);
    // Local-only audio is gitignored, so a fresh worktree lacks it: fall back to
    // the main checkout, and warn (not fail) when neither has the file.
    const source = [file, path.join(mainCheckout, 'data', 'music', `${t.id}.${t.ext}`)].find((f) => existsSync(f));
    if (source) cpSync(source, path.join(musicOut, `${t.id}.${t.ext}`));
    else missing.push(`${t.id}.${t.ext}`);
  }
  if (missing.length) console.log(`[verify-desktop] WARNING: ${missing.length} local-only music track(s) missing, music for them will not play: ${missing.join(', ')}`);
}

// Files over 100 MB are hardlinked instead of copied (same volume, no copy,
// originals untouched by a delete in the copy): delivered MP4s under videos/
// and the job demos. Raw recordings and render-lab bundles are left out.
function linkBigFiles(from, to) {
  let linked = 0;
  const link = (source) => {
    const target = path.join(to, path.relative(from, source));
    if (existsSync(target)) return;
    mkdirSync(path.dirname(target), { recursive: true });
    try {
      linkSync(source, target);
    } catch (error) {
      if (error?.code === 'EXDEV') throw new Error(`CLIPHUB_VERIFY_HOME (${HOME}) must be on the same volume as ${realStudio} to hardlink big files`);
      throw error;
    }
    linked += 1;
  };
  const walk = (dir) => {
    if (!existsSync(dir)) return;
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const source = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (entry.name === 'recording' || entry.name === 'segments') continue;
        walk(source);
      } else if (entry.name.endsWith('.mp4') && /[\\/]videos[\\/]/.test(source) && statSync(source).size > BIG_FILE) {
        link(source);
      }
    }
  };
  walk(path.join(from, 'jobs'));
  const demos = path.join(from, 'demos');
  if (existsSync(demos)) {
    for (const entry of readdirSync(demos, { withFileTypes: true })) {
      const source = path.join(demos, entry.name);
      if (entry.isFile() && entry.name.endsWith('.dem') && statSync(source).size > BIG_FILE) link(source);
    }
  }
  return linked;
}

// robocopy: /R:1 /W:1 so a file the running Studio holds open cannot stall the
// copy (default is a million retries 30 s apart); exit codes below 8 are success.
const ROBOCOPY_QUIET = ['/R:1', '/W:1', '/NFL', '/NDL', '/NJH', '/NJS', '/NP'];
const ROBOCOPY_OK = [0, 1, 2, 3, 4, 5, 6, 7];

// Each copy writes a marker only after it completes, so an interrupted or
// failed copy is redone on the next run instead of being reused half-done.
function ensureProfile() {
  if (!existsSync(realStudio)) throw new Error(`no real Studio data at ${realStudio}`);
  mkdirSync(profile, { recursive: true });
  const tools = path.join(profile, 'tools');
  const toolsReady = path.join(profile, '.tools-ready');
  if (!existsSync(toolsReady)) {
    step('copy tools (ffmpeg, HLAE, yt-dlp)', () => sh('robocopy', [path.join(realStudio, 'tools'), tools, '/E', ...ROBOCOPY_QUIET], { okCodes: ROBOCOPY_OK }));
    writeFileSync(toolsReady, new Date().toISOString());
  }
  const data = path.join(profile, 'data');
  const dataReady = path.join(profile, '.data-ready');
  if (flag('fresh-data')) rmSync(dataReady, { force: true });
  if (!existsSync(dataReady)) {
    if (existsSync(data)) step('drop old data copy', () => rmSync(data, { recursive: true, force: true }));
    step('copy data (<=100 MB files, no lab)', () => sh('robocopy', [path.join(realStudio, 'data'), data, '/E', `/MAX:${BIG_FILE}`, '/XD', 'lab', ...ROBOCOPY_QUIET], { okCodes: ROBOCOPY_OK }));
    const linked = step('hardlink delivered videos and demos', () => linkBigFiles(path.join(realStudio, 'data'), data));
    console.log(`[verify-desktop] ${linked} big file(s) linked`);
    writeFileSync(dataReady, new Date().toISOString());
  }
}

async function run() {
  for (const required of [path.join(desktop, 'dist', 'main.js'), path.join(resources, 'web', 'server.js'), path.join(resources, 'bin', 'zv-orchestrator.exe')]) {
    if (!existsSync(required)) throw new Error(`missing ${required}: run prepare first`);
  }
  stopPreviousApp();
  ensureProfile();
  const require = createRequire(path.join(desktop, 'package.json'));
  const { _electron } = require('playwright-core');
  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const out = path.join(HOME, 'runs', stamp);
  mkdirSync(path.join(out, 'shots'), { recursive: true });
  const runs = readdirSync(path.join(HOME, 'runs')).sort();
  for (const old of runs.slice(0, Math.max(0, runs.length - 10))) rmSync(path.join(HOME, 'runs', old), { recursive: true, force: true });
  const report = { repo, head: sh('git', ['rev-parse', '--short', 'HEAD']).trim(), out, boot_ms: 0, blocked: [], pages: [], check: null };

  const bootStart = Date.now();
  const app = await _electron.launch({
    executablePath: require('electron'),
    args: [path.join(desktop, 'e2e', 'isolated-userdata.cjs')],
    cwd: desktop,
    // No ZV_* from the shell: ZV_BRIDGE_* would let the copy claim real Portal
    // requests, and other overrides would diverge from the installed Studio.
    env: { ...without(process.env, /^(zv_|xai_api_key$)/i), CLIPHUB_E2E_USER_DATA: profile },
  });
  writeFileSync(path.join(HOME, 'app.pid'), String(app.process().pid));
  try {
    // Block writes on the whole context before the first window loads, so boot
    // requests and any later window or popup are covered too.
    if (!flag('allow-writes')) {
      await app.context().route('**/api/**', (route) => {
        const request = route.request();
        const pathname = new URL(request.url()).pathname;
        if (request.method() === 'GET' || pathname.startsWith('/api/session/')) return route.continue();
        report.blocked.push(`${request.method()} ${pathname}`);
        return route.fulfill({ status: 204, body: '' });
      });
    }
    const page = await app.firstWindow();
    const errors = [];
    const failed = [];
    page.on('pageerror', (e) => errors.push(`pageerror: ${String(e).slice(0, 400)}`));
    page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text().slice(0, 400)); });
    page.on('response', (r) => { if (r.status() >= 400 && r.url().includes('/api/')) failed.push(`${r.status()} ${r.request().method()} ${new URL(r.url()).pathname}`); });
    await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/clips(\?.*)?$/, { timeout: 420_000 });
    report.boot_ms = Date.now() - bootStart;
    const origin = new URL(page.url()).origin;
    const skip = page.getByText('Saltar guía', { exact: true });
    if (await skip.waitFor({ timeout: 8000 }).then(() => true, () => false)) {
      await skip.click();
      await skip.waitFor({ state: 'hidden' }).catch(() => {});
    }

    const jobs = await page.evaluate(async () => {
      const response = await fetch('/api/demos/jobs');
      const body = await response.json().catch(() => ({}));
      return (body.jobs ?? []).map((j) => ({ id: j.jobId ?? j.id, status: j.status, map: j.summary?.match?.map ?? null, player: j.summary?.target?.name ?? null }));
    });
    report.jobs = jobs;

    const setWindow = (width, height = 900) => app.evaluate(({ BrowserWindow }, [w, h]) => {
      const win = BrowserWindow.getAllWindows()[0];
      win.setMinimumSize(320, 400);
      win.setContentSize(w, h);
    }, [width, height]);
    const settle = async () => {
      await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
      await page.waitForTimeout(500);
    };
    const shot = async (name, target = page) => {
      const file = path.join(out, 'shots', `${name.replace(/[^a-z0-9._-]+/gi, '_').slice(0, 100)}.png`);
      await target.screenshot({ path: file });
      return file;
    };

    const widths = option('widths', '1440,1024,760').split(',').map(Number).filter(Boolean);
    // Git Bash rewrites a leading "/x" into "C:/Program Files/Git/x"; undo it and accept "x".
    const normalizeRoute = (r) => '/' + r.replace(/^[A-Za-z]:\/.*?\/Git(?=\/)/, '').replace(/^\/+/, '');
    const routes = option('routes', null)?.split(',').map(normalizeRoute) ?? [
      '/clips', '/clips?vista=clips', '/clips/nueva', '/players', '/streams', '/tactical', '/cheaters', '/settings',
      ...jobs.slice(0, 3).flatMap((j) => [`/clips?partida=${j.id}`, `/clips/${j.id}/nuevo?formato=full`, `/clips/${j.id}/nuevo?formato=short`]),
    ];
    if (!flag('no-pass')) {
      for (const width of widths) {
        await setWindow(width);
        for (const route of routes) {
          errors.length = 0; failed.length = 0;
          const start = Date.now();
          await page.goto(origin + route).catch((e) => errors.push(`goto: ${e.message}`));
          await settle();
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth).catch(() => null);
          const errorScreen = await page.evaluate(() => document.body.innerText.includes('no pudo arrancar')).catch(() => false);
          const file = await shot(`${width}${route}`);
          report.pages.push({ width, route, url: page.url().replace(origin, ''), ms: Date.now() - start, overflow, errorScreen, errors: [...errors], failed: [...new Set(failed)], shot: file });
        }
      }
    }

    const checkPath = option('check', null);
    if (checkPath) {
      const log = [];
      const assertions = [];
      const ctx = {
        app, page, origin, jobs, setWindow, settle, shot, log: (...parts) => log.push(parts.join(' ')),
        expect: (ok, message) => { assertions.push({ ok: Boolean(ok), message }); },
      };
      errors.length = 0; failed.length = 0;
      let crash = null;
      try {
        const mod = await import(pathToFileURL(path.resolve(checkPath)).href);
        await mod.default(ctx);
      } catch (error) {
        crash = String(error?.stack ?? error);
        await shot('check-crash').catch(() => {});
      }
      report.check = { file: checkPath, crash, assertions, log, errors: [...errors], failed: [...new Set(failed)] };
    }

    if (flag('keep-open')) {
      console.log(`[verify-desktop] app left open at ${origin} (Ctrl+C to stop)`);
      // Ctrl+C resumes here, so the report, app shutdown and summary still run.
      await new Promise((resolve) => process.once('SIGINT', resolve));
    }
  } finally {
    writeFileSync(path.join(out, 'report.json'), JSON.stringify(report, null, 2));
    await app.close().catch(() => {});
    rmSync(path.join(HOME, 'app.pid'), { force: true });
  }
  return report;
}

// Expected API states that Chromium still logs as console errors. Keep each
// entry narrow and give the reason; anything else is flagged.
const KNOWN = [
  { failed: /^409 GET \/api\/demos\/[^/]+\/anticheat$/, reason: 'anti-cheat analysis not started for that demo' },
];
function dropKnown(page) {
  const failed = page.failed.filter((line) => !KNOWN.some((k) => k.failed.test(line)));
  const knownStatuses = page.failed.filter((line) => !failed.includes(line)).map((line) => line.split(' ')[0]);
  const errors = page.errors.filter((line) => !knownStatuses.some((status) => line.includes(`status of ${status}`)));
  return { ...page, errors, failed, known: page.failed.length - failed.length };
}

function summarize(report) {
  report.pages = report.pages.map(dropKnown);
  writeFileSync(path.join(report.out, 'report.json'), JSON.stringify(report, null, 2));
  const flagged = report.pages.filter((p) => p.overflow > 0 || p.errorScreen || p.errors.length || p.failed.length);
  const known = report.pages.reduce((sum, p) => sum + p.known, 0);
  console.log(`\nhead ${report.head} · boot ${(report.boot_ms / 1000).toFixed(1)}s · ${report.jobs?.length ?? 0} job(s) · ${report.pages.length} page(s), ${flagged.length} flagged, ${known} known · ${report.blocked.length} write(s) blocked`);
  for (const p of flagged) {
    console.log(`  FLAG ${p.width}px ${p.route}${p.overflow > 0 ? ` overflow=${p.overflow}px` : ''}${p.errorScreen ? ' ERROR-SCREEN' : ''}`);
    for (const line of [...p.errors, ...p.failed]) console.log(`       ${line}`);
  }
  if (report.check) {
    const failedAssertions = report.check.assertions.filter((a) => !a.ok);
    console.log(`check ${report.check.file}: ${report.check.assertions.length - failedAssertions.length}/${report.check.assertions.length} ok${report.check.crash ? ' · CRASHED' : ''}`);
    for (const a of report.check.assertions) console.log(`  ${a.ok ? 'ok  ' : 'FAIL'} ${a.message}`);
    for (const line of report.check.log) console.log(`  log ${line}`);
    for (const line of [...report.check.errors, ...report.check.failed]) console.log(`  err ${line}`);
    if (report.check.crash) console.log(report.check.crash);
  }
  console.log(`report ${path.join(report.out, 'report.json')}`);
  const bad = flagged.length > 0 || (report.check && (report.check.crash || report.check.assertions.some((a) => !a.ok)));
  process.exitCode = bad ? 1 : 0;
}

if (mode === 'prepare' || mode === 'all') prepare();
if (mode === 'run' || mode === 'all') summarize(await run());
