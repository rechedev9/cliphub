import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import test from 'node:test';
import { processTreeSpawnOptions, terminateProcessTree } from './process-tree.mjs';
import { captureLabEnvironment } from './safe-env.mjs';
import { protectedCaptureWindow } from './media-oracle.mjs';

const root = resolve(import.meta.dirname, '..', '..');

function node(script, args) {
  return spawnSync(process.execPath, [join(root, 'scripts', 'capturelab', script), ...args], {
    cwd: root, encoding: 'utf8', windowsHide: true,
  });
}

test('subprocess environment drops inherited credentials but permits explicit local capability', () => {
  const environment = captureLabEnvironment({
    PATH: '/tools', HOME: '/home/test', FACEIT_API_KEY: 'faceit', GITHUB_TOKEN: 'github',
    ZV_STEAM_USERNAME: 'steam-user', ORDINARY_VALUE: 'dropped',
    DATABASE_URL: 'postgres://user:pass@example', HTTPS_PROXY: 'http://user:pass@proxy',
    SENTRY_DSN: 'https://secret@sentry', KUBECONFIG: '/secret/kube',
    DOCKER_CONFIG: '/secret/docker', NODE_OPTIONS: '--require=/secret/inject.js',
  }, { ORCHESTRATOR_TOKEN: 'ephemeral-local' });
  assert.equal(environment.PATH, '/tools');
  assert.equal(environment.HOME, '/home/test');
  assert.equal(environment.ORDINARY_VALUE, undefined);
  assert.equal(environment.FACEIT_API_KEY, undefined);
  assert.equal(environment.GITHUB_TOKEN, undefined);
  assert.equal(environment.ZV_STEAM_USERNAME, undefined);
  for (const name of ['DATABASE_URL', 'HTTPS_PROXY', 'SENTRY_DSN', 'KUBECONFIG', 'DOCKER_CONFIG', 'NODE_OPTIONS']) {
    assert.equal(environment[name], undefined, `${name} leaked`);
  }
  assert.equal(environment.ORCHESTRATOR_TOKEN, 'ephemeral-local');
});

test('media oracle independently reproduces settle and near-EOF protected windows', () => {
  assert.deepEqual(protectedCaptureWindow({
    tick_start: 100, tick_end: 700, kills: [{ tick: 500 }],
  }, { tickrate: 64, demo_duration_ticks: 2_000 }), { start: 228, end: 700 });
  assert.deepEqual(protectedCaptureWindow({
    tick_start: 700, tick_end: 950, kills: [{ tick: 900 }],
  }, { tickrate: 64, demo_duration_ticks: 1_000 }), { start: 828, end: 964 });
});

test('process-tree timeout cleanup terminates a spawned grandchild', async () => {
  const source = `
    const { spawn } = require('node:child_process');
    const child = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });
    console.log(child.pid);
    setInterval(() => {}, 1000);
  `;
  const parent = spawn(process.execPath, ['-e', source], {
    stdio: ['ignore', 'pipe', 'ignore'], windowsHide: true, ...processTreeSpawnOptions(),
  });
  const grandchildPID = await new Promise((accept, reject) => {
    let output = '';
    const timer = setTimeout(() => reject(new Error('grandchild PID was not reported')), 5_000);
    parent.stdout.on('data', (chunk) => {
      output += chunk;
      const line = output.split(/\r?\n/)[0];
      if (/^\d+$/.test(line)) {
        clearTimeout(timer);
        accept(Number(line));
      }
    });
    parent.once('error', (error) => {
      clearTimeout(timer);
      reject(error);
    });
  });
  const parentClosed = new Promise((accept) => parent.once('close', accept));
  await terminateProcessTree(parent, 500);
  await parentClosed;
  await new Promise((accept) => setTimeout(accept, 100));
  let alive = true;
  try { process.kill(grandchildPID, 0); } catch (error) { if (error.code === 'ESRCH') alive = false; else throw error; }
  if (alive && process.platform === 'linux') {
    const state = await readFile(`/proc/${grandchildPID}/stat`, 'utf8').catch(() => '');
    if (!state || state.split(' ')[2] === 'Z') alive = false;
  }
  assert.equal(alive, false, `grandchild ${grandchildPID} survived tree cleanup`);
});

test('capture boundary fingerprint is deterministic and names its inputs', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'cliphub-capturelab-tools-'));
  const firstPath = join(dir, 'first.json');
  const secondPath = join(dir, 'second.json');
  let result = node('certificate.mjs', ['fingerprint', '--out', firstPath]);
  assert.equal(result.status, 0, result.stderr);
  result = node('certificate.mjs', ['fingerprint', '--out', secondPath]);
  assert.equal(result.status, 0, result.stderr);
  const [first, second] = await Promise.all([readFile(firstPath, 'utf8').then(JSON.parse), readFile(secondPath, 'utf8').then(JSON.parse)]);
  assert.equal(first.boundary.sha256, second.boundary.sha256);
  assert.ok(first.boundary.files.some((file) => file.path === 'internal/recording/scriptgen.go'));
});

// Encodes one synthetic 64x64 segment clip and returns the oracle's failures for it.
async function oracleFailures({ id, videoSource, audioSource, seconds }) {
  const dir = await mkdtemp(join(tmpdir(), 'cliphub-capturelab-tools-'));
  const video = join(dir, `${id}.mp4`);
  let result = spawnSync('ffmpeg', [
    '-y', '-v', 'error', '-f', 'lavfi', '-i', videoSource,
    '-f', 'lavfi', '-i', audioSource, '-t', String(seconds),
    '-metadata', `comment=cliphub-capturelab:${id}`,
    '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-c:a', 'aac', video,
  ], { encoding: 'utf8', windowsHide: true });
  assert.equal(result.status, 0, result.stderr);
  const recordingResult = join(dir, 'recording-result.json');
  const instrumentation = join(dir, 'capturelab-instrumentation.json');
  await writeFile(recordingResult, JSON.stringify({
    capture_mode: 'fake', capture_verified: false,
    plan: {
      editorial_segment_ids: [id],
      segments: [{ id, tick_start: 0, tick_end: 100, kills: [] }],
      stream: { width: 64, height: 64, fps: 30 },
    },
    artifacts: [{
      segment_id: id, role: 'segment', type: 'video', path: video,
      width: 64, height: 64, frame_rate: '30/1', duration_seconds: seconds,
    }],
  }));
  await writeFile(instrumentation, JSON.stringify({
    schema_version: 1, capture_mode: 'fake',
    segments: [{
      id, path: video, color_rgb: '000000', tone_hz: 440,
      duration_seconds: seconds, event_offsets: [],
    }],
  }));
  // Invoke by a relative path exactly as lab.mjs does. The entry-point check
  // must normalize it to a file URL on both POSIX and Windows.
  result = spawnSync(process.execPath, [
    'scripts/capturelab/media-oracle.mjs',
    '--recording-result', recordingResult, '--instrumentation', instrumentation,
  ], { cwd: root, encoding: 'utf8', windowsHide: true });
  assert.equal(result.status, 2, result.stderr || result.stdout);
  return JSON.parse(result.stdout).failures;
}

test('media oracle rejects frozen and silent synthetic evidence', async (t) => {
  if (spawnSync('ffmpeg', ['-version'], { stdio: 'ignore' }).status !== 0) {
    t.skip('ffmpeg is required for the corruption oracle test');
    return;
  }
  const failures = await oracleFailures({
    id: 'seg-static', seconds: 2,
    videoSource: 'color=black:size=64x64:rate=30:duration=2', audioSource: 'anullsrc=r=48000:cl=mono',
  });
  assert.ok(failures.some((failure) => failure.includes(':motion:')));
  assert.ok(failures.some((failure) => failure.includes(':audio-level:')));
});

test('media oracle rejects a picture that freezes after its two motion samples', async (t) => {
  if (spawnSync('ffmpeg', ['-version'], { stdio: 'ignore' }).status !== 0) {
    t.skip('ffmpeg is required for the corruption oracle test');
    return;
  }
  // Moving for two seconds, then one repeated frame while the audio keeps playing.
  const failures = await oracleFailures({
    id: 'seg-tail', seconds: 3,
    videoSource: 'testsrc2=size=64x64:rate=30:duration=2,tpad=stop_mode=clone:stop_duration=1',
    audioSource: 'sine=frequency=440:sample_rate=48000',
  });
  assert.ok(failures.some((failure) => failure.includes(':frozen-picture:')), failures.join('\n'));
  assert.ok(!failures.some((failure) => failure.includes(':motion:')), 'the 0.5s and 1.5s samples must still differ');
});

test('certificate check fails closed when mandatory evidence is unavailable', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'cliphub-capturelab-tools-'));
  const fingerprintPath = join(dir, 'fingerprint.json');
  let result = node('certificate.mjs', ['fingerprint', '--out', fingerprintPath]);
  assert.equal(result.status, 0, result.stderr);
  const fingerprint = JSON.parse(await readFile(fingerprintPath, 'utf8'));
  const certificatePath = join(dir, 'certificate.json');
  await writeFile(certificatePath, JSON.stringify({
    kind: 'cliphub-real-capture-compatibility-certificate',
    hlae_version: 'test-hlae', cs2_build: 'test-cs2',
    boundary: fingerprint.boundary,
    recording_result: { path: join(dir, 'missing-result.json'), sha256: 'a'.repeat(64) },
    generated_script: { path: join(dir, 'missing-recording.js'), sha256: 'b'.repeat(64) },
    artifacts: [{ segment_id: 'seg-001', path: join(dir, 'missing.mp4'), sha256: 'c'.repeat(64) }],
  }));
  result = node('certificate.mjs', [
    'check', '--certificate', certificatePath,
    '--hlae-version', 'test-hlae', '--cs2-build', 'test-cs2',
  ]);
  assert.equal(result.status, 2, result.stderr || result.stdout);
  const report = JSON.parse(result.stdout);
  assert.ok(report.stale_reasons.includes('recording result is unavailable'));
  assert.ok(report.stale_reasons.includes('generated recording.js is unavailable'));
  assert.ok(report.stale_reasons.includes('segment artifact is unavailable: seg-001'));
});

test('a fake capture can never issue a real compatibility certificate', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'cliphub-capturelab-tools-'));
  const resultPath = join(dir, 'recording-result.json');
  await writeFile(resultPath, JSON.stringify({
    capture_mode: 'fake', capture_verified: false,
    capture_input_fingerprint: 'a'.repeat(64),
    plan: { capture_contract: 'observer-steamid-input-v2' },
  }));
  const argvPath = join(dir, 'argv.json');
  await writeFile(argvPath, JSON.stringify(['zv.exe', 'record']));
  const result = node('certificate.mjs', [
    'issue', '--recording-result', resultPath,
    '--hlae-version', 'test', '--cs2-build', 'test', '--argv-json', argvPath, '--out', join(dir, 'certificate.json'),
  ]);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /capture_mode=real/);
});
