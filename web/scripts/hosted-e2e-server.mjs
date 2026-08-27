import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const web = join(here, '..');
const repo = join(web, '..');
const temporary = mkdtempSync(join(tmpdir(), 'cliphub-hosted-e2e-'));
const webOrigin = 'http://127.0.0.1:3200';
const controlOrigin = 'http://127.0.0.1:8092';
const browserCapability = 'a'.repeat(64);
const children = [];

const environment = {
  ...process.env,
  CLIPHUB_WEB_MODE: 'hosted',
  CLIPHUB_CONTROL_PLANE_URL: controlOrigin,
  PORT: '3200',
  HOSTNAME: '127.0.0.1',
};

const packageManagerScript = process.env.npm_execpath;
if (!packageManagerScript) throw new Error('npm_execpath is required to launch the hosted E2E services');
const build = spawnSync(process.execPath, [packageManagerScript, 'run', 'build'], {
  cwd: web,
  env: environment,
  stdio: 'inherit',
});
if (build.error) throw build.error;
if (build.status !== 0) process.exit(build.status ?? 1);

const control = spawn('go', ['run', './cmd/zv-control-plane'], {
  cwd: repo,
  env: {
    ...process.env,
    CLIPHUB_CONTROL_ADDR: '127.0.0.1:8092',
    CLIPHUB_CONTROL_DB: join(temporary, 'control.db'),
    CLIPHUB_PUBLIC_ORIGIN: webOrigin,
  },
  stdio: 'inherit',
});
children.push(control);

const localGateway = createServer(async (request, response) => {
  process.stdout.write(`[local-gateway] ${request.method} ${request.url}\n`);
  response.setHeader('Access-Control-Allow-Origin', webOrigin);
  response.setHeader('Access-Control-Allow-Methods', 'GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS');
  response.setHeader(
    'Access-Control-Allow-Headers',
    request.headers['access-control-request-headers'] ?? 'Authorization, Content-Type, Range',
  );
  response.setHeader('Access-Control-Allow-Private-Network', 'true');
  response.setHeader('Access-Control-Expose-Headers', 'Content-Range, Accept-Ranges, Content-Length');
  response.setHeader('Cache-Control', 'no-store');
  if (request.method === 'OPTIONS') {
    response.writeHead(204).end();
    return;
  }
  if (request.headers.authorization !== `Bearer ${browserCapability}`) {
    response.writeHead(403, { 'Content-Type': 'application/json' }).end(JSON.stringify({ error: 'capability required' }));
    return;
  }
  const url = new URL(request.url ?? '/', 'http://127.0.0.1:43123');
  if (url.pathname === '/api/capabilities') {
    response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({
      record: {
        enabled: true,
        tools: [
          { name: 'hlae', source: 'detected', configured: true, accessible: true },
          { name: 'cs2', source: 'detected', configured: true, accessible: true },
        ],
      },
      render: {
        enabled: true,
        tools: [{ name: 'ffmpeg', source: 'detected', configured: true, accessible: true }],
      },
      compose: { enabled: true },
      faceit: { enabled: true },
      steam: { enabled: false, gcConfigured: false, historyConfigured: false },
    }));
    return;
  }
  if (url.pathname === '/api/demos/scan') {
    for await (const _chunk of request) {
      // Drain the upload to prove the browser sent it to the loopback gateway.
    }
    response.writeHead(201, { 'Content-Type': 'application/json' }).end(JSON.stringify({ jobId: 'local-e2e-job' }));
    return;
  }
  if (url.pathname.endsWith('/videos/e2e.mp4')) {
    const payload = Buffer.from('local-media');
    response.writeHead(request.headers.range ? 206 : 200, {
      'Content-Type': 'video/mp4',
      'Accept-Ranges': 'bytes',
      'Content-Length': String(payload.length),
      ...(request.headers.range ? { 'Content-Range': `bytes 0-${payload.length - 1}/${payload.length}` } : {}),
    }).end(payload);
    return;
  }
  response.writeHead(404, { 'Content-Type': 'application/json' }).end(JSON.stringify({ error: 'not found' }));
});
localGateway.listen(43123, '127.0.0.1');

await waitForURL(`${controlOrigin}/healthz`, 60_000);
const next = spawn(process.execPath, [packageManagerScript, 'run', 'start'], {
  cwd: web,
  env: environment,
  stdio: 'inherit',
});
children.push(next);

function shutdown() {
  for (const child of children) child.kill();
  localGateway.close();
  rmSync(temporary, { recursive: true, force: true });
}

async function waitForURL(url, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) return;
    } catch {
      // The process is still compiling or binding; retry within the deadline.
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`timed out waiting for ${url}`);
}

process.once('SIGINT', () => { shutdown(); process.exit(0); });
process.once('SIGTERM', () => { shutdown(); process.exit(0); });
process.once('exit', shutdown);

await new Promise(() => {});
