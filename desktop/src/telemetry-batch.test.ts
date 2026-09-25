import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { TelemetryClient } from './telemetry-client.ts';
import { TelemetryJournal } from './telemetry-journal.ts';
import { TelemetrySettingsStore } from './telemetry-settings.ts';

function fixture(t: test.TestContext) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-batch-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const queue = path.join(directory, 'queue.json');
  const errors = path.join(directory, 'errors.jsonl');
  const spans = path.join(directory, 'spans.jsonl');
  const cursor = path.join(directory, 'cursor.json');
  fs.writeFileSync(errors, ''); fs.writeFileSync(spans, '');
  const client = new TelemetryClient({
    settings: new TelemetrySettingsStore(path.join(directory, 'settings.json')),
    queuePath: queue, release: '2.4.57', performanceSampleRate: 1,
    config: { endpoint: 'https://collector.example/ingest', ingestKey: 'test' },
    fetch: async () => ({ ok: false, status: 503 }), log: () => {},
  });
  client.update(true);
  const journal = new TelemetryJournal({
    client, errorJournalPath: errors, spanJournalPath: spans, cursorPath: cursor, log: () => {},
  });
  journal.poll();
  return { client, journal, queue, errors, spans, cursor };
}

const errorLine = `${JSON.stringify({ time: '2026-09-06T12:00:00Z', stage: 'parse', class: 'parse:demo', message: 'parse failed token=private', job_id: '8a46e7a4-d86a-4512-bc41-dc270a296461' })}\n`;
const spanLine = `${JSON.stringify({ time: '2026-09-06T12:00:00Z', stage: 'worker', name: 'parse:demo', result: 'ok', duration_ms: 123 })}\n`;

test('one poll publishes errors and spans once; idle polls do not rewrite the queue', (t) => {
  const f = fixture(t);
  fs.appendFileSync(f.errors, errorLine.repeat(10));
  fs.appendFileSync(f.spans, spanLine.repeat(9));
  f.journal.poll();
  const published = fs.readFileSync(f.queue, 'utf8');
  const events = JSON.parse(published).events;
  assert.equal(events.length, 19);
  assert.equal(events[10].duration_ms, 123);
  assert.doesNotMatch(JSON.stringify(events), /private/);
  assert.match(events[0].message, /parse failed/);
  assert.equal(events[0].job_id, '8a46e7a4-d86a-4512-bc41-dc270a296461');

  // Every queue write renames a fresh temporary file, so a rewrite would move
  // both the inode and the reset modification time.
  fs.utimesSync(f.queue, new Date(0), new Date(0));
  const initial = fs.statSync(f.queue);
  for (let i = 0; i < 5; i++) f.journal.poll();
  const after = fs.statSync(f.queue);
  assert.equal(after.mtimeMs, initial.mtimeMs);
  assert.equal(after.ino, initial.ino);
  assert.equal(fs.readFileSync(f.queue, 'utf8'), published);
});

test('failed queue publication leaves BOTH journal cursors retryable', (t) => {
  const f = fixture(t);
  const before = fs.readFileSync(f.cursor, 'utf8');
  fs.mkdirSync(f.queue);
  fs.appendFileSync(f.errors, errorLine);
  fs.appendFileSync(f.spans, spanLine);
  f.journal.poll();
  assert.equal(fs.readFileSync(f.cursor, 'utf8'), before);
  fs.rmdirSync(f.queue);
  f.journal.poll();
  const events = JSON.parse(fs.readFileSync(f.queue, 'utf8')).events;
  assert.deepEqual(events.map((event: { kind: string }) => event.kind), ['error', 'span']);
  const cursors = JSON.parse(fs.readFileSync(f.cursor, 'utf8'));
  assert.equal(cursors.errors.offset, Buffer.byteLength(errorLine));
  assert.equal(cursors.spans.offset, Buffer.byteLength(spanLine));
  f.journal.poll();
  assert.equal(JSON.parse(fs.readFileSync(f.queue, 'utf8')).events.length, 2);
});

test('batch keeps the bounded tail and revocation prevents later imports', async (t) => {
  const f = fixture(t);
  fs.appendFileSync(f.errors, errorLine.repeat(250));
  fs.appendFileSync(f.spans, spanLine.repeat(10));
  f.journal.poll();
  await f.client.flush();
  const events = JSON.parse(fs.readFileSync(f.queue, 'utf8')).events;
  assert.equal(events.length, 200);
  assert.equal(events.at(-1).kind, 'span');
  f.client.update(false);
  fs.appendFileSync(f.errors, errorLine);
  f.journal.poll();
  assert.equal(fs.existsSync(f.queue), false);
  f.client.update(true);
  f.journal.poll();
  assert.equal(fs.existsSync(f.queue), false);
});
