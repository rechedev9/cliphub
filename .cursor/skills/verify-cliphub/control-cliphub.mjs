#!/usr/bin/env node
import { spawn, spawnSync } from 'node:child_process';
import {
  closeSync,
  existsSync,
  mkdirSync,
  openSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import net from 'node:net';
import { createRequire } from 'node:module';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SKILL_DIR = dirname(fileURLToPath(import.meta.url));
const STATE_PATH = join(SKILL_DIR, '.run', 'state.json');
const DEFAULT_PORT = 4173;
const DEFAULT_HOST = '127.0.0.1';
const READY_PATH = '/clips';
const HUB_EMPTY = '¿Qué quieres crear?';
const HUB_POPULATED = 'Tus demos y vídeos';
const PRODUCT_TITLE = 'ClipHub';
const CLOSED_CAPTURE_GAP = 'hlae_cs2_windows_studio';

const USAGE = `usage: control-cliphub <command> [flags]

Agent control CLI for ClipHub Studio web. Launch an isolated Next server,
doctor the instance, drive one mapped feature, capture proof, then clean up.
Reuse ./bin/zv verify for Windows Studio, HLAE, and running cs2.exe. This
CLI does not recertify capture and does not fake CS2.

Commands:
  launch       start Studio web on an isolated loopback port
  doctor       wrap zv verify doctor and probe the launched web origin
  features     list the feature map ids this CLI can name
  goto         open a Studio path in Chromium
  click        click a role+name control (optional --within)
  snapshot     write an ARIA snapshot
  screenshot   write a PNG
  drive        walk one mapped feature (inicio)
  cleanup      stop the process this launch started; keep evidence
  status       print the current run file

Global flags:
  --json, --format json   machine-readable JSON on stdout
  --help, -h              command help
  --dry-run               print the plan for destructive commands (cleanup)

Launch flags:
  --port <n>              default 4173 (CLIPHUB_VERIFY_PORT)
  --evidence <dir>        proof directory (created if missing)

Doctor flags:
  --dry-run               pass --dry-run to zv verify doctor (no /healthz GET)
  --url <origin>          probe this origin instead of the run file

Drive / browser flags:
  --feature <id>          catalog id (inicio)
  --path <route>          Studio path for goto, default /clips
  --role <role>           ARIA role for click
  --name <name>           accessible name for click
  --within <selector>     scope click to a locator
  --out <path>            snapshot or screenshot destination
  --url <origin>          override the launched origin

Cleanup flags:
  --dry-run               print PIDs and evidence path; do not kill

Evidence survives cleanup. The run file does not. Never pkill next or electron.
Cloud Linux doctor must name ${CLOSED_CAPTURE_GAP}. That is not a web failure.
`;

const COMMAND_USAGE = {
  launch: `usage: control-cliphub launch [--port <n>] [--evidence <dir>] [--json]

Start Studio web with next dev --webpack on 127.0.0.1. Default port 4173.
Writes .cursor/skills/verify-cliphub/.run/state.json with pid, port, and
evidence dir. Refuses a port this run did not start.

Ready: GET /clips returns HTTP 200.
`,
  doctor: `usage: control-cliphub doctor [--json] [--dry-run] [--url <origin>]

Runs zv verify doctor --format json, then probes Studio web.
web.ok is the walk gate. zv.ok is capture recertification and stays false
on Cloud Linux. Expect gap ${CLOSED_CAPTURE_GAP}.
`,
  features: `usage: control-cliphub features [--json] [--feature <id>]

Prints feature ids from this skill map. Cheap inspect, not a user-path walk.
`,
  goto: `usage: control-cliphub goto [--path /clips] [--url <origin>] [--json]

Open a Studio route and wait for the shell to paint.
`,
  click: `usage: control-cliphub click --role <role> --name <name> [--within <sel>] [--json]

Click by ARIA role and accessible name. Prefer this over coordinates.
`,
  snapshot: `usage: control-cliphub snapshot --out <path> [--path /clips] [--json]

Write an ARIA snapshot of the current page (navigates --path first).
`,
  screenshot: `usage: control-cliphub screenshot --out <path> [--path /clips] [--json]

Write a PNG of the current page (navigates --path first).
`,
  drive: `usage: control-cliphub drive --feature inicio [--json]

Walk one mapped feature. inicio opens /clips, asserts the hub, follows
Crear Short, and returns through the Clips y vídeos rail.
`,
  cleanup: `usage: control-cliphub cleanup [--dry-run] [--json]

Kill the PID in the run file and its descendants. Keep the evidence dir.
`,
  status: `usage: control-cliphub status [--json]

Print the run file or report that no instance is recorded.
`,
};

const FEATURES = [
  {
    id: 'inicio',
    title: 'Clips y vídeos',
    route: '/clips',
    file: 'features/inicio.md',
    requires_hlae_cs2: false,
  },
  {
    id: 'clips-de-stream',
    title: 'Clips de stream',
    route: '/streams',
    file: 'features/clips-de-stream.md',
    requires_hlae_cs2: false,
  },
  {
    id: 'jugadores',
    title: 'Jugadores',
    route: '/players',
    file: 'features/jugadores.md',
    requires_hlae_cs2: false,
  },
  {
    id: 'subir-demo',
    title: 'Subir demo',
    route: '/clips/nueva',
    file: 'features/subir-demo.md',
    requires_hlae_cs2: false,
  },
  {
    id: 'demo-completa',
    title: 'Demo completa',
    route: '/clips',
    file: 'features/demo-completa.md',
    requires_hlae_cs2: true,
  },
];

function fail(message, code = 1) {
  process.stderr.write(`error: ${message}\n`);
  process.exit(code);
}

function parseArgs(argv) {
  const flags = {};
  const positionals = [];
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--') {
      positionals.push(...argv.slice(i + 1));
      break;
    }
    if (arg === '-h') {
      flags.help = true;
      continue;
    }
    if (!arg.startsWith('--')) {
      positionals.push(arg);
      continue;
    }
    const trimmed = arg.slice(2);
    const eq = trimmed.indexOf('=');
    const name = eq === -1 ? trimmed : trimmed.slice(0, eq);
    const inline = eq === -1 ? undefined : trimmed.slice(eq + 1);
    if (inline !== undefined) {
      flags[name] = inline;
      continue;
    }
    const next = argv[i + 1];
    if (next && !next.startsWith('-')) {
      flags[name] = next;
      i += 1;
      continue;
    }
    flags[name] = true;
  }
  return { command: positionals[0] ?? '', rest: positionals.slice(1), flags };
}

function wantsJson(flags) {
  return flags.json === true || flags.format === 'json';
}

function printJson(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

function findRepoRoot(start) {
  for (let dir = start; ; dir = dirname(dir)) {
    if (existsSync(join(dir, 'go.mod'))) return dir;
    const parent = dirname(dir);
    if (parent === dir) fail('cliphub repo root not found: missing go.mod', 1);
  }
}

function readState() {
  if (!existsSync(STATE_PATH)) return null;
  try {
    return JSON.parse(readFileSync(STATE_PATH, 'utf8'));
  } catch (err) {
    fail(`run file is not JSON: ${STATE_PATH}: ${err.message}`);
  }
}

function writeState(state) {
  mkdirSync(dirname(STATE_PATH), { recursive: true });
  writeFileSync(STATE_PATH, `${JSON.stringify(state, null, 2)}\n`);
}

function pidAlive(pid) {
  if (!Number.isInteger(pid) || pid <= 0) return false;
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function childPids(pid) {
  const result = spawnSync('pgrep', ['-P', String(pid)], { encoding: 'utf8' });
  if (result.status !== 0) return [];
  return result.stdout
    .split('\n')
    .map((line) => Number(line.trim()))
    .filter((value) => Number.isInteger(value) && value > 0);
}

function collectTree(pid) {
  const seen = new Set();
  const walk = (current) => {
    if (seen.has(current)) return;
    seen.add(current);
    for (const child of childPids(current)) walk(child);
  };
  walk(pid);
  return [...seen];
}

function killTree(pid) {
  const tree = collectTree(pid);
  for (const current of tree) {
    try {
      process.kill(current, 'SIGTERM');
    } catch {}
  }
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline && tree.some((current) => pidAlive(current))) {
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 100);
  }
  for (const current of tree) {
    if (!pidAlive(current)) continue;
    try {
      process.kill(current, 'SIGKILL');
    } catch {}
  }
  return tree;
}

function canListen(port, host) {
  return new Promise((resolveListen) => {
    const server = net.createServer();
    server.unref();
    server.once('error', () => resolveListen(false));
    server.listen(port, host, () => {
      server.close(() => resolveListen(true));
    });
  });
}

async function getJson(url, timeoutMs = 2500) {
  const ac = new AbortController();
  const timer = setTimeout(() => ac.abort(), timeoutMs);
  try {
    const res = await fetch(url, { signal: ac.signal, redirect: 'follow' });
    const text = await res.text();
    return { ok: res.ok, status: res.status, url: res.url, text };
  } catch (err) {
    return { ok: false, status: 0, url, text: '', error: err.message };
  } finally {
    clearTimeout(timer);
  }
}

function zvCommand(repo, args) {
  const built = join(repo, 'bin', 'zv');
  if (existsSync(built)) return { cmd: built, args, cwd: repo };
  return { cmd: 'go', args: ['run', './cmd/zv', ...args], cwd: repo };
}

function runZv(repo, args) {
  const spec = zvCommand(repo, args);
  const result = spawnSync(spec.cmd, spec.args, {
    cwd: spec.cwd,
    encoding: 'utf8',
    env: process.env,
  });
  return {
    bin: spec.cmd === 'go' ? 'go run ./cmd/zv' : spec.cmd,
    args,
    status: result.status ?? 1,
    stdout: result.stdout ?? '',
    stderr: result.stderr ?? '',
    error: result.error ? result.error.message : '',
  };
}

function parseZvJson(raw) {
  const text = raw.trim();
  if (text === '') return null;
  try {
    return JSON.parse(text);
  } catch {
    const start = text.indexOf('{');
    const end = text.lastIndexOf('}');
    if (start >= 0 && end > start) {
      try {
        return JSON.parse(text.slice(start, end + 1));
      } catch {
        return null;
      }
    }
    return null;
  }
}

function evidencePath(state, out) {
  if (!out) fail('missing --out <path>', 2);
  if (isAbsolute(out)) return out;
  const root = state?.evidenceDir ?? join(SKILL_DIR, 'artifacts', 'scratch');
  return resolve(root, out);
}

function writeText(path, body) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, body);
}

async function waitForWeb(origin, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let latest = { ok: false, status: 0, url: `${origin}${READY_PATH}`, error: 'not probed' };
  while (Date.now() < deadline) {
    latest = await getJson(`${origin}${READY_PATH}`, 4000);
    if (latest.ok && latest.status === 200) return latest;
    await new Promise((resolveWait) => setTimeout(resolveWait, 400));
  }
  return latest;
}

function loadPlaywright(repo) {
  const webPkg = join(repo, 'web', 'package.json');
  if (!existsSync(webPkg)) {
    fail('web/package.json is missing; this CLI drives Studio web');
  }
  const requireFromWeb = createRequire(webPkg);
  const candidates = ['playwright', 'playwright-core', '@playwright/test'];
  const errors = [];
  for (const name of candidates) {
    try {
      return requireFromWeb(name);
    } catch (err) {
      errors.push(`${name}: ${err.message}`);
    }
  }
  fail(
    `Playwright is not installed in web/. Run pnpm --dir web install --frozen-lockfile. Tried ${errors.join(
      '; ',
    )}`,
  );
}

async function withPage(repo, origin, path, fn) {
  const playwright = loadPlaywright(repo);
  const chromium = playwright.chromium;
  if (!chromium) fail('Playwright chromium export is missing');
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
  } catch (err) {
    fail(
      `Chromium is not installed for Playwright. Run: pnpm --dir web exec playwright install chromium. ${err.message}`,
    );
  }
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    await page.goto(new URL(path, origin).href, { waitUntil: 'load', timeout: 60_000 });
    await page.locator('body').waitFor({ state: 'visible' });
    await page.waitForFunction(() => Object.keys(document.body).some((key) => key.startsWith('__react')), null, {
      timeout: 30_000,
    });
    return await fn(page);
  } finally {
    await browser.close();
  }
}

async function ariaSnapshot(page) {
  if (typeof page.locator('html').ariaSnapshot === 'function') {
    return page.locator('html').ariaSnapshot();
  }
  return page.evaluate(() => {
    const walk = (node, depth) => {
      const role = node.getAttribute?.('role') ?? node.tagName.toLowerCase();
      const name =
        node.getAttribute?.('aria-label') ??
        node.getAttribute?.('aria-labelledby') ??
        (node.innerText ?? '').trim().split('\n')[0] ??
        '';
      const line = `${'  '.repeat(depth)}${role}${name ? `: ${name.slice(0, 80)}` : ''}`;
      const kids = [...(node.children ?? [])].slice(0, 40).map((child) => walk(child, depth + 1));
      return [line, ...kids].join('\n');
    };
    return walk(document.body, 0);
  });
}

function resolveOrigin(flags, state) {
  if (typeof flags.url === 'string' && flags.url !== '') {
    return flags.url.replace(/\/$/, '');
  }
  if (state?.origin) return state.origin;
  fail('no launched origin. Run control-cliphub launch, or pass --url');
}

function resolvePath(flags, state, fallback = '/clips') {
  if (typeof flags.path === 'string' && flags.path !== '') return flags.path;
  if (typeof state?.lastPath === 'string' && state.lastPath !== '') return state.lastPath;
  return fallback;
}

function requireFeature(id) {
  const feature = FEATURES.find((row) => row.id === id);
  if (!feature) {
    fail(
      `unknown feature ${JSON.stringify(id)}. Known: ${FEATURES.map((row) => row.id).join(', ')}`,
      2,
    );
  }
  return feature;
}

async function cmdLaunch(repo, flags) {
  const port = Number(flags.port ?? process.env.CLIPHUB_VERIFY_PORT ?? DEFAULT_PORT);
  if (!Number.isInteger(port) || port < 1 || port > 65535) fail('--port must be an integer 1-65535', 2);
  const existing = readState();
  if (existing && pidAlive(existing.pid) && existing.port === port) {
    const ready = await waitForWeb(existing.origin, 8_000);
    if (ready.ok) {
      const payload = { ...existing, reused: true, ready };
      if (wantsJson(flags)) printJson(payload);
      else process.stdout.write(`reused ${existing.origin} pid=${existing.pid}\n`);
      return;
    }
  }
  const free = await canListen(port, DEFAULT_HOST);
  if (!free) {
    fail(
      `127.0.0.1:${port} is already taken by a process this run did not start. Pick --port or stop that server. Refusing to drive a shared instance.`,
    );
  }
  const nextBin = join(repo, 'web', 'node_modules', 'next', 'dist', 'bin', 'next');
  if (!existsSync(nextBin)) {
    fail('web/node_modules/next is missing. Run pnpm --dir web install --frozen-lockfile');
  }
  const runId = new Date().toISOString().replace(/[:.]/g, '-');
  const evidenceDir =
    typeof flags.evidence === 'string' && flags.evidence !== ''
      ? resolve(flags.evidence)
      : join(SKILL_DIR, 'artifacts', runId);
  mkdirSync(evidenceDir, { recursive: true });
  mkdirSync(join(SKILL_DIR, '.run'), { recursive: true });
  const logPath = join(SKILL_DIR, '.run', 'web.log');
  const logFd = openSync(logPath, 'w');
  const child = spawn(
    process.execPath,
    [nextBin, 'dev', '--webpack', '--hostname', DEFAULT_HOST, '--port', String(port)],
    {
      cwd: join(repo, 'web'),
      env: { ...process.env, PORT: String(port), HOSTNAME: DEFAULT_HOST },
      detached: true,
      stdio: ['ignore', logFd, logFd],
    },
  );
  closeSync(logFd);
  if (!child.pid) fail('next dev did not start a pid');
  child.unref();
  const origin = `http://${DEFAULT_HOST}:${port}`;
  const state = {
    pid: child.pid,
    port,
    host: DEFAULT_HOST,
    origin,
    evidenceDir,
    logPath,
    runId,
    startedAt: new Date().toISOString(),
    command: [process.execPath, nextBin, 'dev', '--webpack', '--hostname', DEFAULT_HOST, '--port', String(port)],
  };
  writeState(state);
  const ready = await waitForWeb(origin, 180_000);
  if (!ready.ok) {
    killTree(child.pid);
    rmSync(STATE_PATH, { force: true });
    fail(`web did not become ready at ${origin}${READY_PATH}: ${ready.error ?? `HTTP ${ready.status}`}. See ${logPath}`);
  }
  const payload = {
    ...state,
    ready: { ok: ready.ok, status: ready.status, url: ready.url },
  };
  writeText(join(evidenceDir, 'launch.json'), `${JSON.stringify(payload, null, 2)}\n`);
  if (wantsJson(flags)) printJson(payload);
  else process.stdout.write(`ready ${origin}${READY_PATH} pid=${child.pid} evidence=${evidenceDir}\n`);
}

async function cmdDoctor(repo, flags) {
  const zvArgs = ['verify', 'doctor', '--format', 'json'];
  if (flags['dry-run'] === true) zvArgs.push('--dry-run');
  const zvRun = runZv(repo, zvArgs);
  const zv = parseZvJson(zvRun.stdout);
  if (!zv) {
    fail(
      `zv verify doctor did not print JSON (exit ${zvRun.status}). ${zvRun.stderr || zvRun.error || zvRun.stdout}`,
    );
  }
  const state = readState();
  let origin = null;
  if (typeof flags.url === 'string' && flags.url !== '') origin = flags.url.replace(/\/$/, '');
  else if (state?.origin) origin = state.origin;
  let web = { ok: false, status: 'absent', detail: 'no launched origin and no --url' };
  if (origin) {
    const probe = await getJson(`${origin}${READY_PATH}`, 5000);
    const titleMatch = /<title>([^<]*)<\/title>/i.exec(probe.text ?? '');
    const title = titleMatch ? titleMatch[1] : '';
    const named = title.includes(PRODUCT_TITLE);
    web = {
      ok: probe.ok && probe.status === 200 && named,
      status: probe.ok ? 'ok' : 'absent',
      origin,
      http_status: probe.status,
      title,
      detail: named
        ? `GET ${READY_PATH} ${probe.status}`
        : probe.error ?? `GET ${READY_PATH} ${probe.status}; title ${JSON.stringify(title)} does not contain ${PRODUCT_TITLE}`,
    };
  }
  const gaps = Array.isArray(zv.gaps) ? zv.gaps : [];
  const namedClosed = gaps.some((gap) => gap.id === CLOSED_CAPTURE_GAP);
  const report = {
    ok: web.ok,
    closed: Boolean(zv.closed) || namedClosed,
    web,
    zv,
    zv_cli: { bin: zvRun.bin, status: zvRun.status },
    capture_recertification: zv.host?.capture_recertification ?? 'unavailable',
    named_gap: namedClosed ? CLOSED_CAPTURE_GAP : null,
    detail: web.ok
      ? namedClosed
        ? `web is ready. Capture stays closed (${CLOSED_CAPTURE_GAP}).`
        : 'web is ready.'
      : web.detail,
  };
  const evidenceDir = state?.evidenceDir;
  if (evidenceDir) writeText(join(evidenceDir, 'doctor.json'), `${JSON.stringify(report, null, 2)}\n`);
  if (wantsJson(flags)) printJson(report);
  else {
    process.stdout.write(
      `web=${report.web.ok ? 'ok' : 'fail'} zv=${zv.ok ? 'ok' : 'fail'} gap=${report.named_gap ?? 'none'}\n`,
    );
  }
  if (!report.ok) process.exit(1);
}

function rememberPage(state, url) {
  if (!state) return;
  try {
    const parsed = new URL(url);
    state.lastUrl = url;
    state.lastPath = `${parsed.pathname}${parsed.search}`;
    writeState(state);
  } catch {}
}

function cmdFeatures(_repo, flags) {
  const id = typeof flags.feature === 'string' ? flags.feature : '';
  const rows = id === '' ? FEATURES : FEATURES.filter((row) => row.id === id);
  if (id !== '' && rows.length === 0) fail(`unknown feature ${JSON.stringify(id)}`, 2);
  const report = {
    source: '.cursor/skills/verify-cliphub/features',
    catalog: 'internal/verify/catalog.go',
    features: rows,
  };
  if (wantsJson(flags)) printJson(report);
  else {
    for (const row of rows) {
      process.stdout.write(`${row.id}\t${row.route}\t${row.file}\n`);
    }
  }
}

function cmdStatus(_repo, flags) {
  const state = readState();
  if (!state) {
    const report = { ok: false, detail: 'no run file' };
    if (wantsJson(flags)) printJson(report);
    else process.stdout.write('no run file\n');
    process.exit(1);
  }
  const report = { ...state, alive: pidAlive(state.pid) };
  if (wantsJson(flags)) printJson(report);
  else process.stdout.write(`${report.origin} pid=${report.pid} alive=${report.alive} evidence=${report.evidenceDir}\n`);
}

async function cmdGoto(repo, flags) {
  const state = readState();
  const origin = resolveOrigin(flags, state);
  const path = resolvePath(flags, state);
  const result = await withPage(repo, origin, path, async (page) => {
    const title = await page.title();
    const url = page.url();
    return { ok: title.includes(PRODUCT_TITLE), title, url, path };
  });
  rememberPage(state, result.url);
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`${result.url} title=${result.title}\n`);
  if (!result.ok) fail(`page title ${JSON.stringify(result.title)} does not contain ${PRODUCT_TITLE}`);
}

async function cmdClick(repo, flags) {
  const role = flags.role;
  const name = flags.name;
  if (typeof role !== 'string' || role === '') fail('click requires --role', 2);
  if (typeof name !== 'string' || name === '') fail('click requires --name', 2);
  const state = readState();
  const origin = resolveOrigin(flags, state);
  const path = resolvePath(flags, state);
  const within = typeof flags.within === 'string' ? flags.within : '';
  const result = await withPage(repo, origin, path, async (page) => {
    const scope = within === '' ? page : page.locator(within);
    const locator = scope.getByRole(role, { name, exact: flags.exact === true });
    await locator.first().click();
    await page.waitForLoadState('load');
    return { ok: true, role, name, url: page.url(), title: await page.title() };
  });
  rememberPage(state, result.url);
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`clicked ${role}/${JSON.stringify(name)} -> ${result.url}\n`);
}

async function cmdSnapshot(repo, flags) {
  const state = readState();
  const origin = resolveOrigin(flags, state);
  const path = resolvePath(flags, state);
  const out = evidencePath(state, flags.out);
  const result = await withPage(repo, origin, path, async (page) => {
    const aria = await ariaSnapshot(page);
    writeText(out, `${aria}\n`);
    return { ok: true, path: out, url: page.url(), title: await page.title(), bytes: Buffer.byteLength(aria) };
  });
  rememberPage(state, result.url);
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`${out}\n`);
}

async function cmdScreenshot(repo, flags) {
  const state = readState();
  const origin = resolveOrigin(flags, state);
  const path = resolvePath(flags, state);
  const out = evidencePath(state, flags.out);
  const result = await withPage(repo, origin, path, async (page) => {
    mkdirSync(dirname(out), { recursive: true });
    await page.screenshot({ path: out, fullPage: true });
    return { ok: true, path: out, url: page.url(), title: await page.title() };
  });
  rememberPage(state, result.url);
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`${out}\n`);
}

async function cmdDrive(repo, flags) {
  const feature = requireFeature(typeof flags.feature === 'string' ? flags.feature : '');
  if (feature.requires_hlae_cs2) {
    fail(
      `${feature.id} needs Windows Studio, HLAE, and running cs2.exe. Cloud Linux is closed (${CLOSED_CAPTURE_GAP}). Do not fake Pass.`,
    );
  }
  if (feature.id !== 'inicio') {
    fail(`drive implements inicio only. Use goto/click for ${feature.id}, or see features/${feature.id}.md`, 2);
  }
  const state = readState();
  const origin = resolveOrigin(flags, state);
  const evidenceDir = state?.evidenceDir ?? join(SKILL_DIR, 'artifacts', 'scratch');
  mkdirSync(evidenceDir, { recursive: true });
  const result = await withPage(repo, origin, '/clips', async (page) => {
    const steps = [];
    const title = await page.title();
    if (!title.includes(PRODUCT_TITLE)) {
      throw new Error(`hub title ${JSON.stringify(title)} does not contain ${PRODUCT_TITLE}`);
    }
    const current = page.locator('[data-slot="sidebar"] a[aria-current="page"]');
    await current.waitFor({ state: 'visible' });
    const currentHref = await current.getAttribute('href');
    const currentName = (await current.innerText()).replace(/\s+/g, ' ').trim();
    if (currentHref !== '/clips') {
      throw new Error(`rail aria-current href=${JSON.stringify(currentHref)}, want /clips`);
    }
    steps.push({
      id: 'inicio-rail',
      action: 'assert rail Clips y vídeos',
      result: { href: currentHref, name: currentName },
    });

    await page.locator('[aria-label="Cargando partidas"]').waitFor({ state: 'hidden', timeout: 45_000 });
    const empty = page.locator(`section[aria-label="${HUB_EMPTY}"]`);
    const populated = page.getByRole('heading', { name: HUB_POPULATED });
    await empty.or(populated).first().waitFor({ state: 'visible', timeout: 15_000 });
    const emptyVisible = await empty.isVisible().catch(() => false);
    const populatedVisible = await populated.isVisible().catch(() => false);
    if (!emptyVisible && !populatedVisible) {
      throw new Error(`hub shows neither ${JSON.stringify(HUB_EMPTY)} nor ${JSON.stringify(HUB_POPULATED)}`);
    }
    steps.push({
      id: 'inicio-empty',
      action: 'assert hub copy',
      result: { empty: emptyVisible, populated: populatedVisible },
    });

    if (emptyVisible) {
      const shortDoor = page.getByRole('link', { name: /Crear Short/ }).first();
      await shortDoor.waitFor({ state: 'visible' });
      const href = await shortDoor.getAttribute('href');
      if (href !== '/clips/nueva?formato=short') {
        throw new Error(`Crear Short href=${JSON.stringify(href)}`);
      }
      await shortDoor.click();
      await page.waitForURL(/\/clips\/nueva\?formato=short/);
      steps.push({
        id: 'inicio-short-door',
        action: 'click Crear Short',
        result: { url: page.url(), title: await page.title() },
      });
      await page.locator('[data-slot="sidebar-menu-button"][href="/clips"]').click();
      await page.waitForURL(/\/clips(?:\?.*)?$/);
      await page.locator('[aria-label="Cargando partidas"]').waitFor({ state: 'hidden', timeout: 45_000 });
      await empty.or(populated).first().waitFor({ state: 'visible', timeout: 15_000 });
      steps.push({
        id: 'inicio-return',
        action: 'click rail Clips y vídeos',
        result: { url: page.url() },
      });
    }

    const aria = await ariaSnapshot(page);
    const ariaPath = join(evidenceDir, 'hub.aria.txt');
    const pngPath = join(evidenceDir, 'hub.png');
    writeText(ariaPath, `${aria}\n`);
    await page.screenshot({ path: pngPath, fullPage: true });
    if (!aria.includes('ClipHub') && !aria.includes('Clips y vídeos') && !aria.includes(HUB_EMPTY)) {
      throw new Error('ARIA snapshot does not identify ClipHub or the hub');
    }
    return {
      ok: true,
      feature: feature.id,
      origin,
      url: page.url(),
      title: await page.title(),
      steps,
      evidence: { aria: ariaPath, screenshot: pngPath },
    };
  });
  rememberPage(state, result.url);
  const drivePath = join(evidenceDir, 'drive.json');
  writeText(drivePath, `${JSON.stringify(result, null, 2)}\n`);
  result.evidence = { ...result.evidence, drive: drivePath };
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`drove ${feature.id} -> ${result.url}\n`);
}

function cmdCleanup(_repo, flags) {
  const state = readState();
  if (!state) {
    const report = { ok: true, killed: [], evidenceDir: null, detail: 'no run file' };
    if (wantsJson(flags)) printJson(report);
    else process.stdout.write('no run file\n');
    return;
  }
  const tree = pidAlive(state.pid) ? collectTree(state.pid) : [];
  const evidenceExists = existsSync(state.evidenceDir);
  if (flags['dry-run'] === true) {
    const report = {
      ok: true,
      dry_run: true,
      would_kill: tree,
      evidenceDir: state.evidenceDir,
      evidence_exists: evidenceExists,
      state: STATE_PATH,
    };
    if (wantsJson(flags)) printJson(report);
    else process.stdout.write(`would kill ${tree.join(' ') || '(none)'} and keep ${state.evidenceDir}\n`);
    return;
  }
  const killed = pidAlive(state.pid) ? killTree(state.pid) : [];
  rmSync(STATE_PATH, { force: true });
  const after = {
    ok: true,
    killed,
    evidenceDir: state.evidenceDir,
    evidence_exists: existsSync(state.evidenceDir),
    state_removed: !existsSync(STATE_PATH),
  };
  if (state.evidenceDir) writeText(join(state.evidenceDir, 'cleanup.json'), `${JSON.stringify(after, null, 2)}\n`);
  if (wantsJson(flags)) printJson(after);
  else process.stdout.write(`killed ${killed.join(' ') || '(none)'}; evidence ${state.evidenceDir} exists=${after.evidence_exists}\n`);
  if (!after.evidence_exists) fail('cleanup removed evidence; proof artifacts must survive');
}

const COMMANDS = {
  launch: cmdLaunch,
  doctor: cmdDoctor,
  features: cmdFeatures,
  goto: cmdGoto,
  click: cmdClick,
  snapshot: cmdSnapshot,
  screenshot: cmdScreenshot,
  drive: cmdDrive,
  cleanup: cmdCleanup,
  status: cmdStatus,
};

async function main() {
  const { command, flags } = parseArgs(process.argv.slice(2));
  if (command === '' || command === 'help' || flags.help === true) {
    process.stdout.write(COMMAND_USAGE[command] ?? USAGE);
    process.exit(command && !COMMAND_USAGE[command] && command !== 'help' ? 2 : 0);
  }
  if (!COMMANDS[command]) {
    process.stderr.write(`error: unknown command ${JSON.stringify(command)}\n\n`);
    process.stdout.write(USAGE);
    process.exit(2);
  }
  const repo = findRepoRoot(process.cwd());
  await COMMANDS[command](repo, flags);
}

main().catch((err) => {
  fail(err.message || String(err));
});
