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
const HUB_CLIPS_LENS = 'Tus vídeos de demos';
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
  drive        walk one mapped feature (inicio, publicar-video-largo)
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
  --feature <id>          catalog id (inicio, publicar-video-largo)
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
evidence dir. Reuses a live PID on the same port. A different --port
preflights the new port and next install, then stops the prior instance.
--evidence on reuse updates the recorded dir. Refuses a port this run did
not start.

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

Open a Studio route and wait for the shell to paint. Identity is the
ClipHub title suffix or the brand lockup Ir a Clips y vídeos — /clips/nueva
serves tab title Cargar demo without the suffix.
`,
  click: `usage: control-cliphub click --role <role> --name <name> [--within <sel>] [--json]

Click by ARIA role and accessible name. Prefer this over coordinates.
When the control is a link to another Studio path, wait for that URL
before returning. Same-page buttons do not wait for a navigation.
`,
  snapshot: `usage: control-cliphub snapshot --out <path> [--path /clips] [--json]

Write an ARIA snapshot of the current page (navigates --path first).
Waits until hub, Jugadores, streams, upload, and produce loading is
settled (enabled upload chooser; streams without Cargando streams).
Relative --out is from the repo root.
`,
  screenshot: `usage: control-cliphub screenshot --out <path> [--path /clips] [--json]

Write a PNG of the current page (navigates --path first).
Waits until hub, streams, and upload loading is settled. Relative --out
is from the repo root.
`,
  drive: `usage: control-cliphub drive --feature inicio|publicar-video-largo [--json]

Walk one mapped feature. inicio opens /clips, asserts the hub, follows
Crear Short, and returns through the Clips y vídeos rail.
publicar-video-largo opens /clips, follows Publicar on a finished long
video when one exists, or records the honest missing-clip Publicar page.
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
  {
    id: 'publicar-video-largo',
    title: 'Publicar vídeo largo',
    route: '/clips',
    file: 'features/publicar-video-largo.md',
    requires_hlae_cs2: false,
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

function evidencePath(repo, out) {
  if (!out) fail('missing --out <path>', 2);
  if (isAbsolute(out)) return out;
  return resolve(repo, out);
}

function writeText(path, body) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, body);
}

async function waitForHubLoading(page) {
  await page.locator('[aria-label="Cargando partidas"]').waitFor({ state: 'hidden', timeout: 45_000 });
  let pathname = '';
  try {
    pathname = new URL(page.url()).pathname;
  } catch {
    return;
  }
  if (pathname === '/clips') {
    const empty = page.locator(`section[aria-label="${HUB_EMPTY}"]`);
    const populated = page.getByRole('heading', { name: HUB_POPULATED });
    const clipsLens = page.getByRole('heading', { name: HUB_CLIPS_LENS });
    await empty.or(populated).or(clipsLens).first().waitFor({ state: 'visible', timeout: 15_000 });
    return;
  }
  if (pathname === '/players') {
    await page.locator('[aria-label="Cargando jugadores"]').waitFor({ state: 'hidden', timeout: 45_000 });
    return;
  }
  if (/^\/clips\/[^/]+\/nuevo$/.test(pathname)) {
    await page.locator('[aria-label="Cargando la partida"]').waitFor({ state: 'hidden', timeout: 45_000 });
    return;
  }
  if (pathname === '/streams') {
    await waitForStreamsSettled(page);
    return;
  }
  if (pathname === '/clips/nueva') {
    await waitForUploadSettled(page);
    return;
  }
  if (/^\/clips\/[^/]+\/publicar\/[^/]+$/.test(pathname)) {
    await waitForPublishSettled(page);
  }
}

// The page heading is already in the header during the skeleton. Wait for a
// list outcome instead, and fail if Cargando streams is still on screen.
async function waitForStreamsSettled(page) {
  const empty = page.getByText('Tus proyectos aparecerán aquí');
  const listError = page.getByRole('alert').filter({
    has: page.getByRole('button', { name: 'Reintentar' }),
  });
  const counted = page.locator('#stream-projects-title + span');
  await empty.or(listError).or(counted).first().waitFor({ state: 'visible', timeout: 45_000 });
  const loading = page.getByRole('status').filter({ hasText: 'Cargando streams' });
  if (await loading.isVisible().catch(() => false)) {
    throw new Error('Cargando streams is still visible after the settled list signal');
  }
}

// DemoDropzone first-paints interactive=false. The heading is already there;
// the live chooser enables after hydration and is not gated on the orchestrator.
async function waitForUploadSettled(page) {
  const short = page.getByRole('heading', { name: 'Crea un Short' });
  const full = page.getByRole('heading', { name: 'Crea un vídeo largo' });
  await short.or(full).first().waitFor({ state: 'visible', timeout: 15_000 });
  await page
    .locator('input[type="file"][aria-label="Elegir demos de CS2"]:enabled')
    .waitFor({ state: 'attached', timeout: 15_000 });
}

export async function waitForPublishSettled(page) {
  await page.locator('[aria-label="Cargando el clip"]').waitFor({ state: 'hidden', timeout: 45_000 });
  const missing = page.getByRole('heading', { name: /Clip no encontrado|No se pudo cargar el clip/ });
  const templates = page.getByRole('heading', { name: 'Plantillas para vídeo largo', exact: true });
  const shorts = page.getByRole('heading', { name: 'Títulos recomendados', exact: true });
  const failed = page.getByRole('alert').filter({ hasText: 'No se pudo preparar la publicación' });
  const waiting = page.getByText(
    'La preparación para YouTube estará disponible cuando el vídeo esté listo y su revisión resuelta.',
    { exact: true },
  );
  // The assistant header paints before its request starts. Only these outcomes
  // prove that it has finished loading or cannot prepare a draft in this state.
  await missing.or(templates).or(shorts).or(failed).or(waiting).first()
    .waitFor({ state: 'visible', timeout: 45_000 });
  await page.getByRole('status').filter({ hasText: 'Preparando metadatos y horario' })
    .waitFor({ state: 'hidden', timeout: 45_000 });
}

async function cliphubIdentity(page) {
  const title = await page.title();
  if (title.includes(PRODUCT_TITLE)) {
    return { ok: true, title, identity: 'title' };
  }
  const lockup = page.getByRole('link', { name: 'Ir a Clips y vídeos' });
  if ((await lockup.count()) > 0 && (await lockup.first().isVisible().catch(() => false))) {
    return { ok: true, title, identity: 'wordmark' };
  }
  return { ok: false, title, identity: null };
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
    await waitForHubLoading(page);
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
  const requestedEvidence =
    typeof flags.evidence === 'string' && flags.evidence !== '' ? resolve(flags.evidence) : null;
  if (existing && pidAlive(existing.pid) && existing.port === port) {
    const ready = await waitForWeb(existing.origin, 8_000);
    if (ready.ok) {
      if (requestedEvidence && requestedEvidence !== existing.evidenceDir) {
        existing.evidenceDir = requestedEvidence;
        mkdirSync(existing.evidenceDir, { recursive: true });
        writeState(existing);
      }
      const payload = { ...existing, reused: true, ready };
      if (wantsJson(flags)) printJson(payload);
      else process.stdout.write(`reused ${existing.origin} pid=${existing.pid}\n`);
      return;
    }
  }
  const nextBin = join(repo, 'web', 'node_modules', 'next', 'dist', 'bin', 'next');
  if (!existsSync(nextBin)) {
    fail('web/node_modules/next is missing. Run pnpm --dir web install --frozen-lockfile');
  }
  const livePrior = existing && pidAlive(existing.pid);
  if (!livePrior || existing.port !== port) {
    const free = await canListen(port, DEFAULT_HOST);
    if (!free) {
      fail(
        `127.0.0.1:${port} is already taken by a process this run did not start. Pick --port or stop that server. Refusing to drive a shared instance.`,
      );
    }
  }
  if (livePrior) {
    killTree(existing.pid);
    const freedUntil = Date.now() + 10_000;
    const priorHost = existing.host ?? DEFAULT_HOST;
    while (Date.now() < freedUntil && !(await canListen(existing.port, priorHost))) {
      await new Promise((resolveWait) => setTimeout(resolveWait, 150));
    }
  }
  const free = await canListen(port, DEFAULT_HOST);
  if (!free) {
    fail(
      `127.0.0.1:${port} is already taken by a process this run did not start. Pick --port or stop that server. Refusing to drive a shared instance.`,
    );
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
    const identity = await cliphubIdentity(page);
    return { ...identity, url: page.url(), path };
  });
  rememberPage(state, result.url);
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`${result.url} title=${result.title} identity=${result.identity ?? 'none'}\n`);
  if (!result.ok) {
    fail(
      `page does not identify ClipHub (title ${JSON.stringify(result.title)}; missing brand lockup Ir a Clips y vídeos)`,
    );
  }
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
    const locator = scope.getByRole(role, { name, exact: flags.exact === true }).first();
    await locator.waitFor({ state: 'visible' });
    const href = await locator.getAttribute('href');
    const before = page.url();
    await locator.click();
    if (href && !href.startsWith('#') && !href.toLowerCase().startsWith('javascript:')) {
      const target = new URL(href, origin);
      const prior = new URL(before);
      if (target.pathname !== prior.pathname || target.search !== prior.search) {
        await page.waitForURL((url) => {
          return url.pathname === target.pathname && (target.search === '' || url.search === target.search);
        }, { timeout: 15_000 });
      }
    }
    await page.waitForLoadState('domcontentloaded');
    await waitForHubLoading(page);
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
  const out = evidencePath(repo, flags.out);
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
  const out = evidencePath(repo, flags.out);
  const result = await withPage(repo, origin, path, async (page) => {
    mkdirSync(dirname(out), { recursive: true });
    await page.screenshot({ path: out, fullPage: true });
    return { ok: true, path: out, url: page.url(), title: await page.title() };
  });
  rememberPage(state, result.url);
  if (wantsJson(flags)) printJson(result);
  else process.stdout.write(`${out}\n`);
}

const PUBLISH_MISSING_JOB = '11111111-1111-4111-8111-111111111111';
const PUBLISH_MISSING_PATH = `/clips/${PUBLISH_MISSING_JOB}/publicar/${encodeURIComponent(`${PUBLISH_MISSING_JOB}__demo-compilation`)}`;

async function driveInicio(page, origin, evidenceDir) {
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
    feature: 'inicio',
    origin,
    url: page.url(),
    title: await page.title(),
    steps,
    evidence: { aria: ariaPath, screenshot: pngPath },
  };
}

export async function findLongVideoPublish(page) {
  // Only one partida can be open. Snapshot stable IDs rather than repeatedly
  // choosing the first collapsed row (which alternates between two partidas).
  const rowIds = await page.locator('article[id^="partida-"]').evaluateAll((rows) => rows.map((row) => row.id));
  const found = { door: null, expandedRows: 0, inspected: [], publishCount: 0, longPublishCount: 0 };
  for (const id of rowIds) {
    const row = page.locator(`[id="${id}"]`);
    const toggle = row.locator('button[aria-expanded]').first();
    if (await toggle.count() === 0 || await toggle.getAttribute('aria-disabled') === 'true' || await toggle.isDisabled()) {
      found.inspected.push({ id, opened: false, long_publish: 0 });
      continue;
    }
    if (await toggle.getAttribute('aria-expanded') !== 'true') {
      await toggle.click();
    }
    found.expandedRows++;
    await row.locator('button[aria-expanded="true"]').waitFor({ state: 'visible', timeout: 15_000 });
    const label = row.getByText('Vídeos largos · 16:9', { exact: true });
    await label.waitFor({ state: 'visible', timeout: 15_000 });
    // ColumnHead's span -> header -> FullColumn. Broad ancestor matching also
    // includes ShortsColumn, whose Publicar link precedes the long video.
    const longPublish = label.locator('..').locator('..').getByRole('link', { name: 'Publicar', exact: true });
    found.publishCount += await row.getByRole('link', { name: 'Publicar', exact: true }).count();
    found.longPublishCount = await longPublish.count();
    found.inspected.push({ id, opened: true, long_publish: found.longPublishCount });
    if (found.longPublishCount > 0) {
      found.door = longPublish.first();
      break;
    }
  }
  return found;
}

export async function drivePublicarVideoLargo(page, origin, evidenceDir) {
  const steps = [];
  const title = await page.title();
  if (!title.includes(PRODUCT_TITLE)) {
    throw new Error(`hub title ${JSON.stringify(title)} does not contain ${PRODUCT_TITLE}`);
  }
  await page.locator('[aria-label="Cargando partidas"]').waitFor({ state: 'hidden', timeout: 45_000 });
  const empty = page.locator(`section[aria-label="${HUB_EMPTY}"]`);
  const populated = page.getByRole('heading', { name: HUB_POPULATED });
  await empty.or(populated).first().waitFor({ state: 'visible', timeout: 15_000 });
  const emptyVisible = await empty.isVisible().catch(() => false);
  const populatedVisible = await populated.isVisible().catch(() => false);
  const { door, expandedRows, inspected, longPublishCount, publishCount } = await findLongVideoPublish(page);
  steps.push({
    id: 'publicar-hub',
    action: 'open one partida at a time and look for Publicar in that row’s 16:9 column',
    result: {
      empty: emptyVisible,
      populated: populatedVisible,
      expanded_rows: expandedRows,
      inspected_rows: inspected,
      long_publish_links: longPublishCount,
      publish_links: publishCount,
    },
  });

  const hubAriaPath = join(evidenceDir, 'hub.aria.txt');
  const hubPngPath = join(evidenceDir, 'hub.png');
  writeText(hubAriaPath, `${await ariaSnapshot(page)}\n`);
  await page.screenshot({ path: hubPngPath, fullPage: true });

  if (door) {
    const href = await door.getAttribute('href');
    await door.click();
    await page.waitForURL(/\/clips\/[^/]+\/publicar\/[^/]+/);
    await waitForPublishSettled(page);
    steps.push({
      id: 'publicar-open',
      action: 'click Publicar on a finished vídeo largo',
      result: { href, url: page.url() },
    });
  } else {
    await page.goto(new URL(PUBLISH_MISSING_PATH, origin).href, { waitUntil: 'load', timeout: 60_000 });
    await waitForPublishSettled(page);
    steps.push({
      id: 'publicar-missing',
      action: 'open Publicar without a finished long video',
      result: {
        url: page.url(),
        precondition:
          populatedVisible && publishCount > 0
            ? 'hub has Publicar on Shorts only; long-video templates need a finished vídeo largo'
            : 'no Publicar row on this host; templates need a ready long video',
        named_gap: CLOSED_CAPTURE_GAP,
      },
    });
  }

  const templates = page.getByRole('heading', { name: 'Plantillas para vídeo largo' });
  const shortsTitles = page.getByRole('heading', { name: 'Títulos recomendados' });
  const missing = page.getByRole('heading', { name: /Clip no encontrado|No se pudo cargar el clip/ });
  const failed = page.getByRole('alert').filter({
    hasText: 'No se pudo preparar la publicación porque el vídeo falló',
  });
  const requestError = page.getByRole('alert').filter({
    hasText: 'No se pudo preparar la publicación. El MP4 sigue disponible para descargar.',
  });
  const waiting = page.getByText(
    'La preparación para YouTube estará disponible cuando el vídeo esté listo y su revisión resuelta.',
  );
  const templatesVisible = await templates.isVisible().catch(() => false);
  const shortsVisible = await shortsTitles.isVisible().catch(() => false);
  const missingVisible = await missing.isVisible().catch(() => false);
  const failedVisible = await failed.isVisible().catch(() => false);
  const requestErrorVisible = await requestError.isVisible().catch(() => false);
  const waitingVisible = await waiting.isVisible().catch(() => false);
  if (failedVisible && waitingVisible) {
    throw new Error('failed Publicar page also shows waiting copy');
  }
  if (door && shortsVisible) {
    throw new Error('long-video Publicar opened the Shorts assistant instead of templates');
  }

  if (templatesVisible) {
    const firstTemplate = page.getByRole('button', { name: /Usar título recomendado:/ }).first();
    await firstTemplate.waitFor({ state: 'visible' });
    const titleBox = page.getByLabel('Título', { exact: true });
    const descriptionBox = page.getByLabel('Descripción', { exact: true });
    const tagsBox = page.getByLabel('Etiquetas, separadas por comas');
    await firstTemplate.click();
    const appliedTitle = await titleBox.inputValue();
    await titleBox.fill('POV editado en verify');
    await descriptionBox.fill('Descripción editada en verify');
    await tagsBox.fill('CS2, verify');
    await page.getByRole('button', { name: 'Copiar todo', exact: true }).click();
    const copied = await page
      .getByRole('button', { name: 'Copiado' })
      .isVisible()
      .catch(() => false);
    steps.push({
      id: 'publicar-templates',
      action: 'select, edit, and copy long-video templates',
      result: {
        applied_title: appliedTitle,
        edited: true,
        copy: copied ? 'copied' : 'clipboard-unavailable',
      },
    });
  } else {
    steps.push({
      id: 'publicar-settled',
      action: 'record settled Publicar page without inventing templates',
      result: {
        templates: templatesVisible,
        shorts_titles: shortsVisible,
        missing_clip: missingVisible,
        failed: failedVisible,
        request_error: requestErrorVisible,
        waiting: waitingVisible,
        capture_gap: CLOSED_CAPTURE_GAP,
      },
    });
  }

  const aria = await ariaSnapshot(page);
  const ariaPath = join(evidenceDir, 'publicar.aria.txt');
  const pngPath = join(evidenceDir, 'publicar.png');
  writeText(ariaPath, `${aria}\n`);
  await page.screenshot({ path: pngPath, fullPage: true });
  if (
    !aria.includes('ClipHub') &&
    !aria.includes('Clips y vídeos') &&
    !aria.includes('Publicar') &&
    !aria.includes('Clip no encontrado') &&
    !aria.includes('No se pudo cargar el clip')
  ) {
    throw new Error('ARIA snapshot does not identify ClipHub or Publicar');
  }
  return {
    ok: true,
    feature: 'publicar-video-largo',
    origin,
    url: page.url(),
    title: await page.title(),
    templates_reachable: templatesVisible,
    named_gap: CLOSED_CAPTURE_GAP,
    steps,
    evidence: { hub_aria: hubAriaPath, hub_screenshot: hubPngPath, aria: ariaPath, screenshot: pngPath },
  };
}

async function cmdDrive(repo, flags) {
  const feature = requireFeature(typeof flags.feature === 'string' ? flags.feature : '');
  if (feature.requires_hlae_cs2) {
    fail(
      `${feature.id} needs Windows Studio, HLAE, and running cs2.exe. Cloud Linux is closed (${CLOSED_CAPTURE_GAP}). Do not fake Pass.`,
    );
  }
  if (feature.id !== 'inicio' && feature.id !== 'publicar-video-largo') {
    fail(`drive implements inicio and publicar-video-largo. Use goto/click for ${feature.id}, or see features/${feature.id}.md`, 2);
  }
  const state = readState();
  const origin = resolveOrigin(flags, state);
  const evidenceDir = state?.evidenceDir ?? join(SKILL_DIR, 'artifacts', 'scratch');
  mkdirSync(evidenceDir, { recursive: true });
  const result = await withPage(repo, origin, '/clips', async (page) => {
    if (feature.id === 'publicar-video-largo') {
      return drivePublicarVideoLargo(page, origin, evidenceDir);
    }
    return driveInicio(page, origin, evidenceDir);
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

// Import the real driver in browser regression tests without running the CLI.
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((err) => {
    fail(err.message || String(err));
  });
}
