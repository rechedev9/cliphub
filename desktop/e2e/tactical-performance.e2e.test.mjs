// Real isolated Electron -> Next proxy -> Go scan -> binary decode -> canvas replay.
// TEST_DEMO_PATH selects an existing local .dem; no game or capture is launched.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFile } from 'node:child_process';
import { createRequire } from 'node:module';
import { createReadStream, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import test from 'node:test';
import { createE2EProfile } from '../scripts/e2e-profile.mjs';
import { E2E_BOOT_DEADLINE_MS } from '../scripts/e2e-boot-budget.mjs';
import { attachTacticalWorkload } from '../scripts/tactical-workload.mjs';
import { decodePositionsHeader, decodeRoundFrames } from '../../web/lib/tactical-decode.ts';

const require = createRequire(import.meta.url);
const { _electron } = require('playwright-core');
const desktop = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const artifacts = join(desktop, 'e2e', 'artifacts');
const demo = process.env.TEST_DEMO_PATH;
const measure = process.env.CLIPHUB_MEASURE_TACTICAL === '1';
const exec = promisify(execFile);
const delay = (ms) => new Promise((done) => setTimeout(done, ms));

async function poll(read, accept, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  let value;
  while (Date.now() < deadline) {
    value = await read();
    if (accept(value)) return value;
    await delay(250);
  }
  assert.fail(`poll timed out: ${JSON.stringify(value)}`);
}

function startMeasurement(app, scenario) {
  if (!measure) return null;
  const path = join(artifacts, `efficiency-${scenario}-candidate.json`);
  rmSync(`${path}.ready`, { force: true });
  const promise = exec('powershell.exe', [
    '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', join(desktop, '..', 'scripts', 'measure-desktop-efficiency.ps1'),
    '-RootPid', String(app.process().pid), '-Scenario', scenario, '-Seconds', '15', '-OutputPath', path, '-SignalReady', '-SkipGpu',
  ], { timeout: 90_000, windowsHide: true });
  // Attach rejection immediately; propagate it when the workload finishes.
  const completion = promise.then(() => null, (error) => error);
  const finish = async (workload) => {
    const error = await completion;
    if (error) throw error;
    const raw = JSON.parse(readFileSync(path, 'utf8').replace(/^\uFEFF/, ''));
    const report = attachTacticalWorkload(raw, workload);
    writeFileSync(path, JSON.stringify(report, null, 2));
    rmSync(`${path}.ready`, { force: true });
    return report;
  };
  finish.ready = poll(() => existsSync(`${path}.ready`), Boolean, 45_000);
  return finish;
}

test('real demo analysis, verified positions, bounded-round replay, seeks and playback in Electron', {
  skip: demo ? false : 'set TEST_DEMO_PATH to an existing CS2 demo', timeout: 480_000,
}, async (t) => {
  mkdirSync(artifacts, { recursive: true });
  const profile = createE2EProfile('tactical-performance');
  const app = await _electron.launch({
    executablePath: require('electron'), args: [join(desktop, 'e2e', 'isolated-userdata.cjs')],
    cwd: desktop, env: profile.environment(),
  });
  const appProcess = app.process();
  t.after(async () => {
    // Close before removing userData; Windows keeps the live profile locked.
    let timer;
    try {
      await Promise.race([
        app.close().catch(() => {}),
        new Promise((done) => { timer = setTimeout(done, 10_000); }),
      ]);
      if (appProcess.exitCode === null) {
        await exec('taskkill.exe', ['/PID', String(appProcess.pid), '/T', '/F']).catch(() => {});
      }
    } finally {
      clearTimeout(timer);
      profile.dispose();
    }
  });
  const page = await app.firstWindow();
  const errors = [];
  page.on('pageerror', (error) => errors.push(String(error)));
  await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/clips(?:\?.*)?$/, { timeout: E2E_BOOT_DEADLINE_MS });
  await app.evaluate(({ BrowserWindow }) => {
    const win = BrowserWindow.getAllWindows()[0];
    win.setContentSize(1440, 1000); win.show(); win.focus();
  });
  const origin = new URL(page.url()).origin;
  const request = page.context().request;
  const upload = await request.post(`${origin}/api/demos/scan`, {
    multipart: { demo: createReadStream(resolve(demo)) }, timeout: 120_000,
  });
  assert.equal(upload.status(), 201, await upload.text());
  const { jobId } = await upload.json();
  await poll(async () => (await request.get(`${origin}/api/demos/${jobId}/status`)).json(), (state) => {
    assert.notEqual(state.status, 'failed', JSON.stringify(state));
    return state.status === 'scanned' || state.status === 'parsed';
  });
  await page.goto(`${origin}/tactical/${jobId}`);
  await page.getByRole('button', { name: 'ANALIZAR', exact: true }).waitFor();
  const finishAnalysis = startMeasurement(app, 'tactical-analysis');
  await finishAnalysis?.ready;
  const start = performance.now();
  await page.getByRole('button', { name: 'ANALIZAR', exact: true }).click();
  await page.getByRole('slider', { name: 'Posición de la repetición' }).waitFor({ timeout: 180_000 });
  const analysisMS = performance.now() - start;
  const doc = await (await request.get(`${origin}/api/demos/${jobId}/tactical`)).json();
  assert.ok(doc.rounds.length >= 6, 'fixture must have six rounds to exercise LRU eviction');
  const positions = await request.get(`${origin}/api/demos/${jobId}/tactical/positions`);
  assert.equal(positions.status(), 200);
  const bytes = await positions.body();
  assert.equal(createHash('sha256').update(bytes).digest('hex'), doc.positions.sha256);
  const blob = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
  const scale = decodePositionsHeader(blob);
  for (const offset of doc.positions.round_offsets) {
    const frames = decodeRoundFrames(blob, offset, scale);
    assert.equal(frames.length, offset.frame_count);
  }
  writeFileSync(join(artifacts, 'tactical-index.json'), JSON.stringify(doc));
  writeFileSync(join(artifacts, 'tactical-positions.bin'), bytes);
  await finishAnalysis?.({ scenario: 'tactical-analysis', id: `${doc.demo.sha256}:hz${doc.positions.hz}`, operationMS: analysisMS });

  const canvas = page.getByRole('region', { name: /^Repetición de la ronda / }).locator('canvas');
  const scrub = page.getByRole('slider', { name: 'Posición de la repetición' });
  await page.evaluate(() => document.fonts.ready);
  const seek = async (seconds) => {
    await scrub.fill(String(seconds));
    await poll(() => scrub.inputValue(), (value) => Math.abs(Number(value) - seconds) < 0.051, 5000);
  };
  const pixels = () => canvas.evaluate((element) => element.toDataURL());
  await seek(1);
  const initial = await pixels();
  await seek(8); await seek(1);
  assert.equal(await pixels(), initial, 'backward seek changed the rendered frame');
  const positionsRequests = [];
  page.on('request', (req) => { if (req.url().endsWith('/tactical/positions')) positionsRequests.push(req.url()); });
  const rounds = page.getByRole('region', { name: 'Rondas', exact: true }).locator('li > div > button:first-of-type');
  for (let i = 1; i < 6; i++) {
    await rounds.nth(i).click();
    await page.getByRole('region', { name: `Repetición de la ronda ${doc.rounds[i].number}`, exact: true }).waitFor();
    await seek(1);
  }
  await rounds.first().click();
  await page.getByRole('region', { name: `Repetición de la ronda ${doc.rounds[0].number}`, exact: true }).waitFor();
  await seek(1);
  assert.equal(await pixels(), initial, 're-decoding an evicted round changed its pixels');
  assert.equal(positionsRequests.length, 0, 'round switching must not fetch the blob again');

  // Measure cadence externally: no telemetry or performance arrays in production rendering.
  await page.getByRole('button', { name: 'Reproducir (espacio)', exact: true }).click();
  const finishReplay = startMeasurement(app, 'tactical-replay');
  await finishReplay?.ready;
  const durationMS = measure ? 15_000 : 2_000;
  const frames = await page.evaluate((duration) => new Promise((done) => {
    const intervals = [];
    let first; let previous;
    const step = (now) => {
      if (first === undefined) first = now;
      if (previous !== undefined) intervals.push(now - previous);
      previous = now;
      if (now - first >= duration) done(intervals);
      else requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
  }), durationMS);
  assert.ok(frames.length > 10);
  assert.notEqual(await scrub.inputValue(), '1', 'transport did not advance');
  const display = await page.evaluate(() => ({
    width: innerWidth, height: innerHeight, dpr: devicePixelRatio,
    canvasWidth: document.querySelector('canvas[aria-hidden="true"]').width,
    canvasHeight: document.querySelector('canvas[aria-hidden="true"]').height,
  }));
  await finishReplay?.({ scenario: 'tactical-replay', id: `${doc.demo.sha256}:round${doc.rounds[0].number}:hz${doc.positions.hz}:speed1:${JSON.stringify(display)}`, frameIntervalsMS: frames });
  await page.getByRole('button', { name: 'Pausar (espacio)', exact: true }).click();
  await scrub.press('ArrowRight');
  await scrub.press('Shift+ArrowLeft');
  await page.screenshot({ path: join(artifacts, 'tactical-replay.png') });
  assert.deepEqual(errors, []);
  writeFileSync(join(artifacts, 'tactical-verification.json'), JSON.stringify({
    rounds: doc.rounds.length, positionBytes: bytes.length, analysisMS, display,
    frameSamples: frames.length, blobSHA256: doc.positions.sha256, pageErrors: errors,
  }, null, 2));
});
