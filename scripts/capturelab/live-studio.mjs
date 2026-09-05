#!/usr/bin/env node
import { randomBytes } from 'node:crypto';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { createWriteStream } from 'node:fs';
import { createServer } from 'node:net';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import { processTreeSpawnOptions, terminateProcessTree } from './process-tree.mjs';
import { captureLabEnvironment } from './safe-env.mjs';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

function parseArgs(args) {
  const options = { timeoutMS: 600_000 };
  for (let index = 0; index < args.length; index++) {
    const flag = args[index];
    const value = args[++index];
    if (!value || value.startsWith('--')) throw new Error(`${flag} requires a value`);
    if (flag === '--seed') options.seed = resolve(value);
    else if (flag === '--evidence-dir') options.evidenceDir = resolve(value);
    else if (flag === '--timeout-seconds') options.timeoutMS = Number(value) * 1000;
    else throw new Error(`unknown argument ${flag}`);
  }
  if (!options.seed || !options.evidenceDir) throw new Error('--seed and --evidence-dir are required');
  return options;
}

async function freePort() {
  return await new Promise((accept, reject) => {
    const server = createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      const port = typeof address === 'object' && address ? address.port : 0;
      server.close((error) => error ? reject(error) : accept(port));
    });
  });
}

const maxCapturedOutputBytes = 8 * 1024 * 1024;

function boundedAppend(current, chunk) {
  const combined = Buffer.concat([current, chunk]);
  return combined.length <= maxCapturedOutputBytes ? combined : combined.subarray(combined.length - maxCapturedOutputBytes);
}

async function command(executable, args, options = {}) {
  return await new Promise((accept, reject) => {
    const child = spawn(executable, args, {
      cwd: options.cwd ?? repoRoot, env: captureLabEnvironment(process.env, options.env ?? {}),
      stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true,
      ...processTreeSpawnOptions(),
    });
    let stdout = Buffer.alloc(0);
    let stderr = Buffer.alloc(0);
    child.stdout.on('data', (chunk) => { stdout = boundedAppend(stdout, chunk); });
    child.stderr.on('data', (chunk) => { stderr = boundedAppend(stderr, chunk); });
    let termination = Promise.resolve();
    const timer = setTimeout(() => {
      termination = terminateProcessTree(child);
    }, options.timeoutMS ?? 600_000);
    child.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
    child.once('close', async (code) => {
      clearTimeout(timer);
      await termination;
      const result = {
        code: code ?? -1,
        stdout: stdout.toString('utf8'),
        stderr: stderr.toString('utf8'),
      };
      if (result.code !== 0) reject(new Error(`${executable} exited ${result.code}:\n--- stdout ---\n${result.stdout}\n--- stderr ---\n${result.stderr}`));
      else accept(result);
    });
  });
}

async function waitForAPI(url, token, timeoutMS) {
  const deadline = Date.now() + timeoutMS;
  let lastError = '';
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${url}/api/jobs`, { headers: { 'X-ClipHub-Token': token } });
      if (response.ok) return;
      lastError = `HTTP ${response.status}: ${await response.text()}`;
    } catch (error) {
      lastError = error.message;
    }
    await new Promise((accept) => setTimeout(accept, 100));
  }
  throw new Error(`orchestrator did not become ready: ${lastError}`);
}

async function stopProcess(child) {
  if (!child?.pid) return;
  await terminateProcessTree(child, 10_000);
}

const options = parseArgs(process.argv.slice(2));
await mkdir(options.evidenceDir, { recursive: true });
const seed = JSON.parse(await readFile(options.seed, 'utf8'));
const sourcePlan = JSON.parse(await readFile(seed.killplan_path, 'utf8'));
const token = randomBytes(32).toString('hex');
const [apiPort, webPort] = await Promise.all([freePort(), freePort()]);
const apiURL = `http://127.0.0.1:${apiPort}`;
const executable = join(options.evidenceDir, process.platform === 'win32' ? 'zv-orchestrator.exe' : 'zv-orchestrator');
const build = await command('go', ['build', '-tags', 'capturelab', '-o', executable, './cmd/zv-orchestrator'], { timeoutMS: options.timeoutMS });
await writeFile(join(options.evidenceDir, 'orchestrator-build.log'), `${build.stdout}${build.stderr}`, 'utf8');
const orchestratorLog = createWriteStream(join(options.evidenceDir, 'orchestrator.log'), { flags: 'w' });
let orchestrator;
try {
  orchestrator = spawn(executable, [], {
    cwd: repoRoot,
    env: captureLabEnvironment(process.env, {
      ZV_DATABASE_URL: 'memory',
      ZV_HTTP_ADDR: `127.0.0.1:${apiPort}`,
      ZV_DATA_DIR: join(options.evidenceDir, 'studio-data'),
      ZV_MUTATION_TOKEN: token,
      ZV_CAPTURE_LAB_SEED: options.seed,
      ZV_CAPTURE_LAB_EVIDENCE_ROOT: dirname(options.seed),
      FACEIT_API_KEY: '', FIRECRAWL_API_KEY: '',
      ZV_STEAM_USERNAME: '', ZV_STEAM_PASSWORD: '', ZV_STEAM_GUARD: '',
    }),
    stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true,
    ...processTreeSpawnOptions(),
  });
  orchestrator.stdout.pipe(orchestratorLog, { end: false });
  orchestrator.stderr.pipe(orchestratorLog, { end: false });
  await waitForAPI(apiURL, token, 30_000);
  const expectedVideo = seed.expected_video_path;
  if (!expectedVideo) throw new Error('seed.expected_video_path is required for the browser download oracle');
  // Windows package-manager shims require cmd; this command is entirely
  // repository-owned and contains no seed paths, tokens or user arguments.
  const playwrightArgs = ['--dir', 'web', 'exec', 'playwright', 'test', 'e2e/capture-lab-live.spec.ts', '--reporter=line'];
  const playwrightExecutable = process.platform === 'win32' ? 'cmd.exe' : 'pnpm';
  const playwrightCommand = process.platform === 'win32'
    ? ['/d', '/s', '/c', `pnpm ${playwrightArgs.join(' ')}`]
    : playwrightArgs;
  const playwright = await command(playwrightExecutable, playwrightCommand, {
    timeoutMS: options.timeoutMS,
    env: {
      ORCHESTRATOR_URL: apiURL,
      ORCHESTRATOR_TOKEN: token,
      E2E_PORT: String(webPort),
      CAPTURE_LAB_LIVE: '1',
      CAPTURE_LAB_JOB_ID: seed.job_id,
      CAPTURE_LAB_VARIANT: seed.variant,
      CAPTURE_LAB_SEGMENT_IDS: JSON.stringify(sourcePlan.segments.map((segment) => segment.id)),
      CAPTURE_LAB_EXPECTED_VIDEO: expectedVideo,
      PLAYWRIGHT_OUTPUT_DIR: join(options.evidenceDir, 'live-playwright-results'),
      PLAYWRIGHT_HTML_OUTPUT_DIR: join(options.evidenceDir, 'live-playwright-report'),
    },
  });
  await writeFile(join(options.evidenceDir, 'live-playwright.log'), `${playwright.stdout}${playwright.stderr}`, 'utf8');
  process.stdout.write(playwright.stdout);
} finally {
  await stopProcess(orchestrator);
  await new Promise((accept) => orchestratorLog.end(accept));
}
