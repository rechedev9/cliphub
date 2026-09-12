import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test, { type TestContext } from 'node:test';
import { DiagnosticLogClient, type DiagnosticLogClientOptions } from './diagnostic-log-client.ts';
import { type DiagnosticLogRecord } from './diagnostic-log.ts';
import { TelemetrySettingsStore } from './telemetry-settings.ts';
import { ProcessLogLines } from './process-log-lines.ts';

function fixture(t: TestContext, overrides: Partial<DiagnosticLogClientOptions> = {}, eligible = true) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-debug-'));
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  if (eligible) settings.update(true);
  const sent: DiagnosticLogRecord[][] = [];
  const local: string[] = [];
  const options: DiagnosticLogClientOptions = {
    directory: path.join(directory, 'logs'), settings, release: '3.0.2', sessionID: randomUUID(),
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'test-ingest-key-not-a-real-credential' },
    localLog: (text) => local.push(text),
    fetch: async (_url, init) => {
      assert.equal(init.redirect, 'error');
      const records = (JSON.parse(init.body) as { records: DiagnosticLogRecord[] }).records;
      sent.push(records);
      return { status: 202, json: async () => ({ accepted_ids: records.map((record) => record.id), inserted: records.length }) };
    }, ...overrides,
  };
  const client = new DiagnosticLogClient(options);
  const clients = [client];
  t.after(() => { for (const item of clients) item.stop(); fs.rmSync(directory, { recursive: true, force: true }); });
  const restart = (changes: Partial<DiagnosticLogClientOptions> = {}) => {
    clients.at(-1)?.stop();
    const next = new DiagnosticLogClient({ ...options, sessionID: randomUUID(), ...changes });
    clients.push(next);
    return next;
  };
  return { client, directory, options, settings, sent, local, restart };
}

async function drain(client: DiagnosticLogClient): Promise<void> {
  for (let n = 0; n < 200 && client.status().pendingFiles > 0; n++) await client.flush();
  assert.equal(client.status().pendingFiles, 0, JSON.stringify(client.status()));
}

test('the durable log retains the full causal tail, filters before persistence and correlates the attempt', async (t) => {
  const f = fixture(t);
  const job = randomUUID();
  const attempt = randomUUID();
  const message = 'Preparing render\n' + 'frame preparation observation\n'.repeat(700)
    + 'FINAL CAUSE: encoder device unavailable; token=private-token; demo=C:\\Users\\Alice\\private.dem';
  f.client.recordText(`[orchestrator] [cliphub-diagnostic] ${JSON.stringify({ event: 'attempt.finished', level: 'error', message, job_id: job, attempt_id: attempt, operation: 'render:variant', attempt: 2, outcome: 'error', duration_ms: 1400 })}\n`);
  const persisted = fs.readdirSync(f.options.directory).filter((name) => name.endsWith('.jsonl'))
    .map((name) => fs.readFileSync(path.join(f.options.directory, name), 'utf8')).join('');
  assert.doesNotMatch(persisted, /private-token|Alice|private\.dem/);
  assert.match(persisted, /FINAL CAUSE: encoder device unavailable/);
  await drain(f.client);
  const records = f.sent.flat();
  assert.ok(records.length >= 2, 'long context must be chunked without becoming a 2 KiB summary');
  assert.ok(records.every((r) => r.job_id === job && r.attempt_id === attempt && r.operation === 'render:variant'));
  assert.ok(records.every((r) => Buffer.byteLength(r.message) <= 16 * 1024));
  assert.equal(records[0]?.event, 'attempt.finished');
  assert.match(records.map((r) => r.message).join(''), /FINAL CAUSE: encoder device unavailable/);
  assert.ok(f.client.status().lastAcknowledgedAt);
});

test('lost receipts retain identical record IDs across restart and offline appends', async (t) => {
  const received: DiagnosticLogRecord[] = [];
  const f = fixture(t, { fetch: async (_url, init) => {
    received.push(...(JSON.parse(init.body) as { records: DiagnosticLogRecord[] }).records);
    throw new Error('connection reset after server commit');
  } });
  f.client.record({ message: 'first causal failure', event: 'attempt.finished', level: 'error' });
  await f.client.flush();
  assert.equal(received.length, 1);
  assert.ok(f.client.status().pendingBytes > 0);
  f.client.record({ message: 'context written while offline' });
  const retry = f.restart({ fetch: async (_url, init) => {
    const records = (JSON.parse(init.body) as { records: DiagnosticLogRecord[] }).records;
    received.push(...records);
    return { status: 202, json: async () => ({ accepted_ids: records.map((r) => r.id) }) };
  } });
  await drain(retry);
  assert.equal(received[0]?.id, received[1]?.id, 'retries must not invent new record identities');
  assert.equal(received[0]?.sequence, received[1]?.sequence);
  assert.equal(received[2]?.message, 'context written while offline');
});

test('a bare or mismatched 202 response cannot erase pending evidence', async (t) => {
  const f = fixture(t, { fetch: async () => ({ status: 202, json: async () => ({ accepted_ids: [randomUUID()] }) }) });
  f.client.record({ message: 'must survive an invalid receipt' });
  await f.client.flush();
  assert.ok(f.client.status().pendingBytes > 0);
  assert.match(f.client.status().lastError ?? '', /receipt_mismatch/);
  const next = f.restart({ fetch: async (_url, init) => ({ status: 202, json: async () => ({ accepted_ids: JSON.parse(init.body).records.map((r: DiagnosticLogRecord) => r.id) }) }) });
  await drain(next);
});

test('server failure and an old collector keep the spool for a later delivery', async (t) => {
  for (const status of [404, 429, 503]) {
    const f = fixture(t, { fetch: async () => ({ status, json: async () => ({ code: 'unavailable' }) }) });
    f.client.record({ message: `failure during HTTP ${status}` });
    await f.client.flush();
    assert.ok(f.client.status().pendingBytes > 0);
    assert.equal(f.client.status().rejectedRecords, 0);
    assert.match(f.client.status().lastError ?? '', new RegExp(`collector_http_${status}`));
  }
});

test('the diagnostic choice blocks collection and revocation cancels an in-flight receipt', async (t) => {
  const f = fixture(t, {}, false);
  f.client.record({ message: 'before the informed choice' });
  assert.equal(f.client.status().pendingFiles, 0);
  f.settings.update(true);
  f.client.resetConsent(true);
  let release: (() => void) | undefined;
  const waiting = new Promise<void>((resolve) => { release = resolve; });
  let sawSignal: AbortSignal | undefined;
  const client = f.restart({ fetch: async (_url, init) => {
    sawSignal = init.signal;
    await waiting;
    return { status: 202, json: async () => ({ accepted_ids: JSON.parse(init.body).records.map((r: DiagnosticLogRecord) => r.id) }) };
  } });
  client.record({ message: 'eligible diagnostic' });
  const flushing = client.flush();
  f.settings.update(false);
  client.resetConsent(false);
  assert.equal(sawSignal?.aborted, true);
  release?.();
  await flushing;
  client.record({ message: 'must not upload after revocation' });
  assert.equal(client.status().pendingFiles, 0);
  assert.equal(client.status().lastAcknowledgedAt, null);
  f.settings.update(true);
  client.resetConsent(true);
  assert.equal(client.status().pendingFiles, 0, 're-enabling must not replay revoked logs');
});

test('capacity loss is explicit and the latest diagnostic still reaches the collector', async (t) => {
  const f = fixture(t, { maxSpoolBytes: 512 * 1024 });
  for (let n = 0; n < 850; n++) f.client.record({ message: `observation ${n} ` + 'bounded context '.repeat(65) });
  f.client.record({ level: 'error', event: 'attempt.finished', message: 'latest root cause: encoder device lost' });
  assert.ok(f.client.status().droppedRecords > 0);
  assert.ok(f.client.status().pendingBytes < 600 * 1024);
  await drain(f.client);
  const records = f.sent.flat();
  assert.ok(records.some((r) => r.event === 'delivery.gap' && (r.lost_records ?? 0) > 0));
  assert.ok(records.some((r) => r.message.includes('latest root cause: encoder device lost')));
});

test('a crash-truncated record does not pin the spool or conceal loss', async (t) => {
  const f = fixture(t);
  f.client.record({ message: 'intact error before crash' });
  const file = fs.readdirSync(f.options.directory).find((name) => name.endsWith('.jsonl'));
  assert.ok(file);
  fs.appendFileSync(path.join(f.options.directory, file), '{"message":"unfinished');
  const next = f.restart();
  next.record({ message: 'next session can still diagnose errors' });
  await drain(next);
  const records = f.sent.flat();
  assert.ok(records.some((r) => r.message === 'intact error before crash'));
  assert.ok(records.some((r) => r.message === 'next session can still diagnose errors'));
  assert.ok(records.some((r) => r.event === 'delivery.gap'));
});

test('a rejected record retains its original causal text in a visible rejection event', async (t) => {
  let rejected = false;
  const f = fixture(t, { fetch: async (_url, init) => {
    const records = JSON.parse(init.body).records as DiagnosticLogRecord[];
    if (!rejected) { rejected = true; return { status: 422, json: async () => ({ code: 'invalid_log' }) }; }
    f.sent.push(records);
    return { status: 202, json: async () => ({ accepted_ids: records.map((r) => r.id) }) };
  } });
  f.client.record({ message: 'actual failing encoder detail', event: 'process.stderr' });
  await drain(f.client);
  assert.equal(f.client.status().rejectedRecords, 1);
  assert.ok(f.sent.flat().some((r) => r.event === 'delivery.rejected' && r.message.includes('actual failing encoder detail')));
});

test('pipe chunk boundaries cannot split source tags, UTF-8 or an authorization value', async (t) => {
  const f = fixture(t);
  const lines = new ProcessLogLines((line) => f.client.recordText(`[orchestrator] ${line}`));
  const text = Buffer.from('error español 🙂 authorization: Bearer private-pipe-value\nfinal encoder cause\n');
  for (let offset = 0; offset < text.length; offset++) lines.write(text.subarray(offset, offset + 1));
  lines.end();
  await drain(f.client);
  const combined = f.sent.flat().map((r) => r.message).join('\n');
  assert.match(combined, /español 🙂/);
  assert.match(combined, /final encoder cause/);
  assert.doesNotMatch(combined, /private-pipe-value|�/);
});

test('unwritable spool initialization stays fail-open and exposes its diagnostic failure', (t) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-debug-unwritable-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const blocked = path.join(directory, 'not-a-directory');
  fs.writeFileSync(blocked, 'occupied');
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  settings.update(true);
  const logs: string[] = [];
  const client = new DiagnosticLogClient({ directory: blocked, settings, sessionID: randomUUID(), release: '3.0.2',
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'fixture' }, localLog: (message) => logs.push(message) });
  t.after(() => client.stop());
  assert.doesNotThrow(() => client.record({ message: 'disk error must not crash Studio' }));
  assert.ok(client.status().lastError);
  assert.ok(logs.length > 0);
});

test('an evicted in-flight segment cannot resurrect its cursor or stall newer evidence', async (t) => {
  let release: (() => void) | undefined;
  const waiting = new Promise<void>((resolve) => { release = resolve; });
  let first = true;
  const f = fixture(t, { maxSpoolBytes: 512 * 1024, fetch: async (_url, init) => {
    const records = JSON.parse(init.body).records as DiagnosticLogRecord[];
    if (first) { first = false; await waiting; }
    f.sent.push(records);
    return { status: 202, json: async () => ({ accepted_ids: records.map((r) => r.id) }) };
  } });
  f.client.record({ message: 'in-flight evidence' });
  const flushing = f.client.flush();
  for (let n = 0; n < 800; n++) f.client.record({ message: `context ${n} ` + 'encoder preparation '.repeat(60) });
  f.client.record({ message: 'latest causal failure' });
  release?.();
  await flushing;
  await drain(f.client);
  assert.ok(f.sent.flat().some((r) => r.event === 'delivery.gap'));
  assert.ok(f.sent.flat().some((r) => r.message === 'latest causal failure'));
  assert.equal(JSON.parse(fs.readFileSync(path.join(f.options.directory, 'state.json'), 'utf8')).cursor, null);
});

test('a stale network rejection after a consent reset cannot poison the new delivery state', async (t) => {
  let release: (() => void) | undefined;
  const waiting = new Promise<void>((resolve) => { release = resolve; });
  const f = fixture(t, { fetch: async () => { await waiting; throw new Error('old request aborted'); } });
  f.client.record({ message: 'old eligible evidence' });
  const flushing = f.client.flush();
  f.settings.update(false);
  f.client.resetConsent(false);
  f.settings.update(true);
  f.client.resetConsent(true);
  f.client.record({ message: 'new eligible evidence' });
  release?.();
  await flushing;
  assert.equal(f.client.status().lastError, null);
  assert.equal(f.client.status().lastAcknowledgedAt, null);
  const next = f.restart({ fetch: async (_url, init) => {
    const records = JSON.parse(init.body).records as DiagnosticLogRecord[];
    f.sent.push(records);
    return { status: 202, json: async () => ({ accepted_ids: records.map((r) => r.id) }) };
  } });
  await drain(next);
  assert.deepEqual(f.sent.flat().map((r) => r.message), ['new eligible evidence']);
});

test('missing consent metadata never replays an unproven old spool', async (t) => {
  const f = fixture(t);
  f.client.record({ message: 'old spool with lost consent metadata' });
  fs.writeFileSync(path.join(f.options.directory, 'state.json'), '{unfinished');
  const next = f.restart();
  next.record({ message: 'current authorized diagnostic' });
  await drain(next);
  assert.ok(f.sent.flat().some((r) => r.event === 'delivery.gap'));
  assert.ok(f.sent.flat().some((r) => r.message === 'current authorized diagnostic'));
  assert.ok(f.sent.flat().every((r) => r.message !== 'old spool with lost consent metadata'));
});
