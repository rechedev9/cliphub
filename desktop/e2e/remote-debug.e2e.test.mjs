import assert from 'node:assert/strict';
import { execFileSync, spawn } from 'node:child_process';
import { randomBytes, randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { DiagnosticLogClient } from '../src/diagnostic-log-client.ts';
import { ProcessSession } from '../src/process-session.ts';
import { TelemetrySettingsStore } from '../src/telemetry-settings.ts';
import { collectDiagnostics } from '../../scripts/telemetry-debug.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

test('a real failing subprocess is diagnosable using only collector logs, through the Studio delivery modules', { timeout: 180_000 }, async (t) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-remote-debug-e2e-'));
  const extension = process.platform === 'win32' ? '.exe' : '';
  const collectorPath = path.join(directory, `collector${extension}`);
  const workerPath = path.join(directory, `workers.test${extension}`);
  execFileSync('go', ['build', '-o', collectorPath, './services/telemetry'], { cwd: root, windowsHide: true, stdio: 'pipe' });
  execFileSync('go', ['test', '-c', '-o', workerPath, './internal/workers'], { cwd: root, windowsHide: true, stdio: 'pipe' });
  const ingestKey = randomBytes(32).toString('hex');
  const adminToken = randomBytes(32).toString('hex');
  const collector = spawn(collectorPath, [], { windowsHide: true, env: {
    ...process.env,
    CLIPHUB_TELEMETRY_DATABASE: path.join(directory, 'telemetry.db'),
    CLIPHUB_TELEMETRY_PUBLIC_ADDR: '127.0.0.1:0', CLIPHUB_TELEMETRY_ADMIN_ADDR: '127.0.0.2:0',
    CLIPHUB_TELEMETRY_INGEST_KEY: ingestKey, CLIPHUB_TELEMETRY_ADMIN_TOKEN: adminToken,
    CLIPHUB_TELEMETRY_PROXY_PROTOCOL: '',
  }, stdio: ['ignore', 'pipe', 'pipe'] });
  t.after(async () => {
    if (collector.exitCode === null) { collector.kill(); await new Promise((resolve) => collector.once('close', resolve)); }
    fs.rmSync(directory, { recursive: true, force: true });
  });
  const addresses = await new Promise((resolve, reject) => {
    let text = '';
    const timeout = setTimeout(() => reject(new Error(`Collector startup timeout: ${text}`)), 20_000);
    const read = (chunk) => {
      text += chunk.toString();
      const match = /class=started public=(127\.0\.0\.1:\d+) admin=(127\.0\.0\.2:\d+)/.exec(text);
      if (match) { clearTimeout(timeout); resolve({ public: `http://${match[1]}`, admin: `http://${match[2]}` }); }
    };
    collector.stderr.on('data', read);
    collector.stdout.on('data', read);
    collector.once('error', (error) => { clearTimeout(timeout); reject(error); });
    collector.once('exit', (code) => { clearTimeout(timeout); reject(new Error(`Collector exited ${code}: ${text}`)); });
  });
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  settings.update(true);
  const spool = path.join(directory, 'spool');
  let diagnosticLogs = new DiagnosticLogClient({ directory: spool, settings, release: '3.0.2', sessionID: randomUUID(),
    config: { endpoint: `${addresses.public}/v1/ingest`, ingestKey }, allowInsecureLoopback: true,
    localLog: () => {},
    // Simulate losing the first receipt *after* the real collector committed.
    fetch: async (url, options) => { const response = await fetch(url, options); assert.equal(response.status, 202); throw new Error('simulated receipt loss after durable commit'); },
  });
  const localLog = path.join(directory, 'studio.log');
  const session = new ProcessSession({ logLine: (text) => { fs.appendFileSync(localLog, text); diagnosticLogs.recordText(text); } });
  t.after(() => { diagnosticLogs.stop(); session.stop(); });
  const jobID = randomUUID();
  const worker = session.launch('orchestrator', workerPath, ['-test.run=^TestDiagnosticEndToEndCanary$'], {
    CLIPHUB_DIAGNOSTIC_END_TO_END: '1', CLIPHUB_DIAGNOSTIC_JOB_ID: jobID,
  });
  await worker.exited.catch(() => {});
  await worker.outputClosed;
  assert.ok(diagnosticLogs.status().pendingBytes > 0);
  await diagnosticLogs.flush();
  assert.ok(diagnosticLogs.status().pendingBytes > 0, 'lost receipt must leave the spool intact');
  diagnosticLogs.stop();
  diagnosticLogs = new DiagnosticLogClient({ directory: spool, settings, release: '3.0.2', sessionID: randomUUID(),
    config: { endpoint: `${addresses.public}/v1/ingest`, ingestKey }, allowInsecureLoopback: true, localLog: () => {},
  });
  for (let n = 0; n < 100 && diagnosticLogs.status().pendingFiles > 0; n++) await diagnosticLogs.flush();
  assert.equal(diagnosticLogs.status().pendingFiles, 0);
  // The verification below has no local-log fallback.
  fs.unlinkSync(localLog);
  const report = await collectDiagnostics({ job_id: jobID }, async (url) => {
    const response = await fetch(new URL(url, addresses.admin), { headers: { Authorization: `Bearer ${adminToken}` } });
    assert.equal(response.status, 200);
    return response.json();
  });
  assert.equal(report.evidence.retrieval_complete, true);
  assert.equal(report.attempts.length, 1);
  assert.equal(report.attempts[0].start_record_present, true);
  assert.equal(report.attempts[0].state, 'error');
  assert.ok(report.records.some((r) => r.message.includes('CANARY_CAUSE: encoder device unavailable')));
  assert.ok(report.records.some((r) => r.event === 'process.finished' && r.exit_code === 17));
  assert.ok(report.records.some((r) => r.event === 'attempt.finished' && r.message === 'exit status 1'));
  assert.ok(report.evidence.failure_context.some((r) => r.message.includes('CANARY_CAUSE: encoder device unavailable')));
  assert.equal(new Set(report.records.map((r) => r.id)).size, report.records.length, 'lost receipt retry duplicated stored evidence');
  const artifacts = path.join(root, '.local', 'remote-debug-validation');
  fs.mkdirSync(artifacts, { recursive: true });
  fs.writeFileSync(path.join(artifacts, 'collector-only-canary.json'), JSON.stringify(report, null, 2));
});
