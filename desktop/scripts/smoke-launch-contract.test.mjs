import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import {
  chmodSync,
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, dirname, join, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

test('PowerShell smoke waits for and retains the delegated listener identity', () => {
  const source = readFileSync(join(repo, 'scripts', 'smoke-real.ps1'), 'utf8');
  const waitStart = source.indexOf('for ($attempt = 0; $attempt -lt 40; $attempt++)');
  const waitEnd = source.indexOf('$capabilitiesRaw =', waitStart);
  const waitLoop = source.slice(waitStart, waitEnd);

  assert.match(waitLoop, /Get-LoopbackListenerProcessID/);
  assert.match(waitLoop, /Test-ProcessBelongsToLaunch/);
  assert.match(waitLoop, /\$candidateServer = Get-Process/);
  assert.match(waitLoop, /\[void\]\$candidateServer\.Handle/);
  assert.ok(
    waitLoop.indexOf('Test-ProcessBelongsToLaunch') <
      waitLoop.indexOf('$ownedServer = $candidateServer'),
    'listener ancestry must be verified before adoption',
  );
  assert.match(waitLoop, /Assert-OwnedListener[\s\S]*health response ownership check/);
  assert.match(
    source,
    /capabilities response ownership check[\s\S]*\$capabilities = \$capabilitiesRaw/,
  );
  assert.doesNotMatch(waitLoop, /HasExited\)\s*\{\s*break/);
  assert.match(
    source,
    /Test-ProcessBelongsToLaunch[\s\S]*taskkill\.exe \/PID \$ownedServer\.Id \/T \/F/,
  );
});

test('PowerShell smoke rejects a post-preflight decoy listener without killing it', async (t) => {
  if (process.platform !== 'win32') {
    t.skip('PowerShell listener ownership is Windows-specific');
    return;
  }

  const root = mkdtempSync(join(tmpdir(), 'cliphub-smoke-decoy-race-'));
  const scriptsDir = join(root, 'scripts');
  const binDir = join(root, 'bin');
  const smokePath = join(scriptsDir, 'smoke-real.ps1');
  const fakeZV = join(binDir, 'zv.exe');
  const demo = join(root, 'fixture.dem');
  const outDir = join(root, 'out');
  mkdirSync(scriptsDir, { recursive: true });
  mkdirSync(binDir, { recursive: true });
  writeFileSync(smokePath, readFileSync(join(repo, 'scripts', 'smoke-real.ps1')));
  copyFileSync(process.execPath, fakeZV);
  writeFileSync(join(root, 'serve'), 'setInterval(() => {}, 1_000);\n');
  writeFileSync(demo, 'not parsed because listener ownership must fail first');

  const reservation = spawn(process.execPath, [
    '-e',
    `
const net = require('node:net');
const server = net.createServer();
server.listen(0, '127.0.0.1', () => {
  process.stdout.write(String(server.address().port));
});
`,
  ], { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
  const port = await new Promise((resolvePort, rejectPort) => {
    reservation.once('error', rejectPort);
    reservation.once('exit', (status) => {
      rejectPort(new Error(`port reservation exited before startup with status ${status}`));
    });
    reservation.stdout.once('data', (chunk) => {
      resolvePort(Number.parseInt(String(chunk), 10));
    });
  });
  reservation.kill();
  await new Promise((resolveExit) => reservation.once('exit', resolveExit));

  let smoke;
  let decoy;
  t.after(() => {
    if (smoke?.exitCode === null) smoke.kill();
    if (decoy?.exitCode === null) decoy.kill();
    rmSync(root, { recursive: true, force: true });
  });

  smoke = spawn(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      smokePath,
      '-Demo',
      demo,
      '-OutDir',
      outDir,
      '-OrchestratorPort',
      String(port),
    ],
    {
      cwd: root,
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
    },
  );

  let stdout = '';
  let stderr = '';
  const startup = new Promise((resolveStartup, rejectStartup) => {
    const timeout = setTimeout(
      () => rejectStartup(new Error(`PowerShell smoke did not launch its wrapper:\n${stdout}\n${stderr}`)),
      10_000,
    );
    smoke.stdout.on('data', (chunk) => {
      stdout += String(chunk);
      if (stdout.includes('Started isolated smoke orchestrator')) {
        clearTimeout(timeout);
        resolveStartup();
      }
    });
    smoke.stderr.on('data', (chunk) => {
      stderr += String(chunk);
    });
    smoke.once('error', rejectStartup);
    smoke.once('exit', (status) => {
      clearTimeout(timeout);
      rejectStartup(new Error(`PowerShell smoke exited before wrapper startup with status ${status}`));
    });
  });
  await startup;

  decoy = spawn(
    process.execPath,
    [
      '-e',
      `
const http = require('node:http');
const port = Number.parseInt(process.argv[1], 10);
const server = http.createServer((request, response) => {
  response.writeHead(200, { 'content-type': 'application/json' });
  response.end(request.url === '/healthz' ? '{"ok":true}' : '{}');
});
server.listen(port, '127.0.0.1', () => process.stdout.write('READY'));
`,
      String(port),
    ],
    { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true },
  );
  await new Promise((resolveReady, rejectReady) => {
    decoy.once('error', rejectReady);
    decoy.once('exit', (status) => {
      rejectReady(new Error(`decoy listener exited before startup with status ${status}`));
    });
    decoy.stdout.once('data', (chunk) => {
      if (String(chunk).includes('READY')) resolveReady();
    });
  });

  const smokeStatus = smoke.exitCode ?? await new Promise((resolveExit, rejectExit) => {
      const timeout = setTimeout(() => {
        smoke.kill();
        rejectExit(new Error(`PowerShell smoke did not reject the decoy:\n${stdout}\n${stderr}`));
      }, 15_000);
      smoke.once('error', rejectExit);
      smoke.once('exit', (status) => {
        clearTimeout(timeout);
        resolveExit(status);
      });
    });

  assert.notEqual(smokeStatus, 0, stdout);
  assert.match(
    `${stdout}\n${stderr}`,
    new RegExp(`smoke port ${port} became owned by unrelated process ${decoy.pid}`),
  );
  assert.equal(decoy.exitCode, null, 'the post-preflight decoy listener must remain alive');
  const response = await fetch(`http://127.0.0.1:${port}/healthz`);
  assert.equal(response.status, 200);
});

test('shell smoke does not treat wrapper exit as listener startup failure', () => {
  const source = readFileSync(join(repo, 'scripts', 'smoke.sh'), 'utf8').replaceAll('\r\n', '\n');
  const waitStart = source.indexOf('for _ in $(seq 1 40); do');
  const waitEnd = source.indexOf('echo "→ started isolated smoke orchestrator', waitStart);
  const waitLoop = source.slice(waitStart, waitEnd);

  assert.match(waitLoop, /CURRENT_LISTENER_PID=.*listener_pid/);
  assert.match(waitLoop, /process_belongs_to_launch[\s\S]*"\$OWNED_PID_STARTED_TICKS"/);
  assert.match(waitLoop, /windows_process_started_ticks "\$CURRENT_LISTENER_PID"/);
  assert.match(waitLoop, /OWNED_SERVER_STARTED_TICKS="\$CURRENT_LISTENER_STARTED_TICKS"/);
  assert.match(waitLoop, /OWNED_SERVER_PID="\$CURRENT_LISTENER_PID"/);
  assert.ok(
    waitLoop.indexOf('process_belongs_to_launch') <
      waitLoop.indexOf('OWNED_SERVER_PID="$CURRENT_LISTENER_PID"'),
    'listener ownership must be verified before adoption',
  );
  assert.doesNotMatch(waitLoop, /kill -0 "\$OWNED_PID"/);
  assert.match(source, /windows_taskkill_if_same "\$OWNED_SERVER_PID" "\$OWNED_SERVER_STARTED_TICKS"/);
});

test('Windows netstat listener selection ignores non-TCP and non-LISTENING rows', (t) => {
  const gitBash = join(
    process.env.ProgramFiles ?? 'C:\\Program Files',
    'Git',
    'bin',
    'bash.exe',
  );
  if (!existsSync(gitBash)) {
    t.skip('Git Bash is unavailable');
    return;
  }

  const source = readFileSync(join(repo, 'scripts', 'smoke.sh'), 'utf8').replaceAll('\r\n', '\n');
  const filter = source.match(
    /awk -v suffix=":\$port" \\\r?\n\s*'([^']+)'/,
  )?.[1];
  assert.ok(filter, 'netstat awk filter must be extractable');
  const result = spawnSync(
    gitBash,
    ['-c', `awk -v suffix=":18080" '${filter}'`],
    {
      encoding: 'utf8',
      input: [
        'TCP 127.0.0.1:18080 127.0.0.1:52000 ESTABLISHED 111',
        'UDP 127.0.0.1:18080 *:* LISTENING 222',
        'TCP 127.0.0.1:18080 0.0.0.0:0 LISTENING 333',
        '',
      ].join('\n'),
    },
  );

  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), '333');
});

test('Windows shell cleanup holds creation identity and refuses a reused wrapper PID', async (t) => {
  if (process.platform !== 'win32') {
    t.skip('Windows process identity is Windows-specific');
    return;
  }

  const source = readFileSync(join(repo, 'scripts', 'smoke.sh'), 'utf8').replaceAll('\r\n', '\n');
  const helperStart = source.indexOf('windows_taskkill_if_same()');
  assert.notEqual(helperStart, -1);
  const helperEnd = source.slice(helperStart).search(/\n}\r?\n\r?\nprocess_belongs_to_launch\(\)/);
  assert.notEqual(helperEnd, -1);
  const helper = source.slice(helperStart, helperStart + helperEnd);
  const powershell = helper.match(
    /powershell\.exe -NoProfile -NonInteractive -Command '\r?\n([\s\S]*?)\r?\n\s*' >\/dev\/null/,
  )?.[1];
  assert.ok(powershell, 'identity-checked taskkill script must be extractable');
  assert.ok(
    powershell.indexOf('[void]$process.Handle') <
      powershell.indexOf('Start-Process `'),
    'the original process handle must stay open before taskkill starts',
  );
  assert.ok(
    powershell.indexOf('$actualTicks -ne $env:FF_PROCESS_STARTED_TICKS') <
      powershell.indexOf('Start-Process `'),
    'creation identity must be checked before taskkill starts',
  );
  assert.doesNotMatch(source, /taskkill\.exe \/PID "\$OWNED_PID"/);

  const decoy = spawn(
    process.execPath,
    ['-e', 'setInterval(() => {}, 1_000);'],
    { stdio: 'ignore', windowsHide: true },
  );
  t.after(() => {
    if (decoy.exitCode === null) decoy.kill();
  });

  const result = spawnSync(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-Command', powershell],
    {
      encoding: 'utf8',
      env: {
        ...process.env,
        FF_PROCESS_PID: String(decoy.pid),
        // Deliberately stale identity simulates numeric PID reuse.
        FF_PROCESS_STARTED_TICKS: '1',
      },
    },
  );

  assert.equal(result.status, 3, result.stderr);
  assert.equal(decoy.exitCode, null, 'an identity mismatch must not terminate the process');
  process.kill(decoy.pid, 0);
});

test('Windows listener ancestry rejects reused ancestor PIDs but permits an exited wrapper', () => {
  const source = readFileSync(join(repo, 'scripts', 'smoke.sh'), 'utf8').replaceAll('\r\n', '\n');
  const ancestryStart = source.indexOf('process_belongs_to_launch()');
  const ancestryEnd = source.indexOf('\n}\n\ncleanup()', ancestryStart);
  const ancestry = source.slice(ancestryStart, ancestryEnd);

  assert.match(ancestry, /FF_LAUNCH_STARTED_TICKS/);
  assert.match(ancestry, /FF_LAUNCH_NOT_BEFORE_TICKS/);
  assert.match(ancestry, /\$currentStartedTicks -gt \$descendantStartedTicks/);
  assert.match(ancestry, /\$descendantStartedTicks -lt \$launchNotBeforeTicks/);
  assert.match(ancestry, /\$currentRootTicks -eq \$rootStartedTicks/);
  assert.match(ancestry, /\$currentRootTicks -gt \$descendantStartedTicks/);
  assert.match(
    source,
    /OWNED_PID_STARTED_TICKS="\$\([\s\S]*windows_process_started_ticks "\$OWNED_PID"[\s\S]*\)" \|\| OWNED_PID_STARTED_TICKS=""/,
  );
  assert.ok(
    ancestry.indexOf('$currentRootTicks -gt $descendantStartedTicks') <
      ancestry.indexOf('exit 0', ancestry.indexOf('$currentRootTicks -gt $descendantStartedTicks')),
    'only a root PID reused after the listener was created may preserve exited-wrapper ancestry',
  );
});

test('Windows ancestry accepts a listener child after its short wrapper exits', async (t) => {
  if (process.platform !== 'win32') {
    t.skip('Windows process ancestry is Windows-specific');
    return;
  }

  const source = readFileSync(join(repo, 'scripts', 'smoke.sh'), 'utf8').replaceAll('\r\n', '\n');
  const ancestryStart = source.indexOf('process_belongs_to_launch()');
  const ancestryEnd = source.indexOf('\n}\n\ncleanup()', ancestryStart);
  const ancestry = source.slice(ancestryStart, ancestryEnd);
  const powershell = ancestry.match(
    /powershell\.exe -NoProfile -NonInteractive -Command '\r?\n([\s\S]*?)\r?\n\s*' >\/dev\/null/,
  )?.[1];
  assert.ok(powershell, 'ancestry PowerShell must be extractable');

  const windowsEpochTicks = 621_355_968_000_000_000n;
  const launchNotBefore = windowsEpochTicks + BigInt(Date.now()) * 10_000n;
  const wrapper = spawn(
    process.execPath,
    [
      '-e',
      `
const { spawn } = require('node:child_process');
const child = spawn(
  process.execPath,
  ['-e', 'setInterval(() => {}, 1_000);'],
  { detached: true, stdio: 'ignore', windowsHide: true },
);
child.unref();
process.stdout.write(String(child.pid));
setTimeout(() => process.exit(0), 500);
`,
    ],
    { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true },
  );
  const listenerPID = await new Promise((resolvePID, rejectPID) => {
    wrapper.once('error', rejectPID);
    wrapper.once('exit', (status) => {
      rejectPID(new Error(`short wrapper exited before reporting its child (${status})`));
    });
    wrapper.stdout.once('data', (chunk) => {
      resolvePID(Number.parseInt(String(chunk), 10));
    });
  });
  t.after(() => {
    try {
      process.kill(listenerPID);
    } catch (error) {
      if (error?.code !== 'ESRCH') throw error;
    }
    if (wrapper.exitCode === null) wrapper.kill();
  });
  await new Promise((resolveExit, rejectExit) => {
    if (wrapper.exitCode !== null) {
      resolveExit();
      return;
    }
    wrapper.once('error', rejectExit);
    wrapper.once('exit', resolveExit);
  });

  const result = spawnSync(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-Command', powershell],
    {
      encoding: 'utf8',
      env: {
        ...process.env,
        FF_LAUNCH_NOT_BEFORE_TICKS: String(launchNotBefore),
        FF_LAUNCH_PID: String(wrapper.pid),
        // This is the QA-217 contract: the wrapper vanished before its exact
        // creation identity could be captured.
        FF_LAUNCH_STARTED_TICKS: '',
        FF_LISTENER_PID: String(listenerPID),
      },
    },
  );

  assert.equal(result.status, 0, result.stderr);
  process.kill(listenerPID, 0);
});

test('shell smoke rejects an occupied Unix port without killing its listener', async (t) => {
  const gitBash = join(
    process.env.ProgramFiles ?? 'C:\\Program Files',
    'Git',
    'bin',
    'bash.exe',
  );
  if (!existsSync(gitBash)) {
    t.skip('Git Bash is unavailable');
    return;
  }

  const root = mkdtempSync(join(tmpdir(), 'cliphub-smoke-occupied-port-'));
  const demo = join(root, 'fixture.dem');
  const shimDir = join(root, 'shims');
  const lsofShim = join(shimDir, 'lsof');
  mkdirSync(shimDir, { recursive: true });
  writeFileSync(demo, 'not parsed because preflight must fail');
  writeFileSync(
    lsofShim,
    '#!/usr/bin/env bash\nprintf \'%s\\n\' "$DECOY_PID"\n',
    { mode: 0o755 },
  );
  chmodSync(lsofShim, 0o755);

  const decoy = spawn(
    process.execPath,
    [
      '-e',
      `
const http = require('node:http');
const server = http.createServer((_request, response) => {
  response.writeHead(200, { 'content-type': 'application/json' });
  response.end('{"status":"parsed","id":"decoy"}');
});
server.listen(0, '127.0.0.1', () => {
  process.stdout.write(String(server.address().port) + '\\n');
});
`,
    ],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );
  const port = await new Promise((resolvePort, rejectPort) => {
    decoy.once('error', rejectPort);
    decoy.once('exit', (status) => {
      rejectPort(new Error(`decoy server exited before startup with status ${status}`));
    });
    decoy.stdout.once('data', (chunk) => {
      resolvePort(Number.parseInt(String(chunk).trim(), 10));
    });
  });

  t.after(() => {
    if (decoy.exitCode === null) {
      decoy.kill();
    }
    rmSync(root, { recursive: true, force: true });
  });

  const gitRoot = resolve(dirname(gitBash), '..');
  const result = spawnSync(
    gitBash,
    [
      join(repo, 'scripts', 'smoke.sh').replaceAll('\\', '/'),
      demo.replaceAll('\\', '/'),
    ],
    {
      cwd: repo,
      encoding: 'utf8',
      env: {
        ...process.env,
        DECOY_PID: String(decoy.pid),
        PATH: [shimDir, join(gitRoot, 'usr', 'bin')].join(delimiter),
        ZV_SMOKE_HTTP_ADDR: `127.0.0.1:${port}`,
      },
      timeout: 10_000,
    },
  );

  assert.equal(result.error, undefined);
  assert.notEqual(result.status, 0, result.stdout);
  assert.match(result.stderr, new RegExp(`smoke port ${port} is already owned by process ${decoy.pid}`));
  assert.equal(decoy.exitCode, null, 'the pre-existing listener must remain alive');
  const response = await fetch(`http://127.0.0.1:${port}/healthz`);
  assert.equal(response.status, 200);
});
