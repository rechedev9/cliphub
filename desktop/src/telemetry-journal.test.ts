import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { TelemetryClient } from './telemetry-client.ts';
import { TelemetryJournal } from './telemetry-journal.ts';
import { TelemetrySettingsStore } from './telemetry-settings.ts';

test('journal startup stays fail-open when an ineligible cursor cannot persist', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-journal-fail-'));
  const errors = path.join(directory, 'obs', 'journal.jsonl');
  const queue = path.join(directory, 'queue.json');
  const unwritableCursorTarget = path.join(directory, 'cursor-is-a-directory');
  fs.mkdirSync(path.dirname(errors), { recursive: true });
  fs.mkdirSync(unwritableCursorTarget);
  fs.writeFileSync(errors, `${JSON.stringify({
    time: '2026-08-29T12:00:00Z', stage: 'render', class: 'render:variant', message: 'local only',
  })}\n`);
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  const logs: string[] = [];
  const client = new TelemetryClient({
    settings,
    queuePath: queue,
    release: '2.4.35',
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'public-ingest-key-with-24-characters' },
    fetch: async () => ({ ok: true, status: 202 }),
    log: (message) => logs.push(message),
  });
  const journal = new TelemetryJournal({
    client,
    errorJournalPath: errors,
    spanJournalPath: path.join(directory, 'obs', 'spans.jsonl'),
    cursorPath: unwritableCursorTarget,
    log: (message) => logs.push(message),
  });
  assert.doesNotThrow(() => journal.start());
  journal.stop();
  client.update(true);
  journal.poll();
  assert.equal(fs.existsSync(queue), false);
  assert.ok(logs.some((message) => message.includes('cursor deferred')));
});

test('journal importer discards pre-notice events and never reads sensitive error fields', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-journal-'));
  const errors = path.join(directory, 'obs', 'journal.jsonl');
  const spans = path.join(directory, 'obs', 'spans.jsonl');
  const queue = path.join(directory, 'queue.json');
  fs.mkdirSync(path.dirname(errors), { recursive: true });
  fs.writeFileSync(spans, `${JSON.stringify({
    time: '2026-08-29T11:59:00Z',
    stage: 'worker',
    name: 'parse:demo',
    result: 'ok',
    duration_ms: 10,
  })}\n`);
  fs.writeFileSync(errors, `${JSON.stringify({
    time: '2026-08-29T12:00:00Z',
    stage: 'render',
    class: 'render:variant',
    message: 'C:\\Users\\Luis\\secret.dem token=abc',
    demo: 'secret.dem',
    target_steamid: '76561198000000000',
  })}\n`);
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  const client = new TelemetryClient({
    settings,
    queuePath: queue,
    release: '2.4.35',
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'public-ingest-key-with-24-characters' },
    fetch: async () => ({ ok: true, status: 202 }),
    log: () => {},
    performanceSampleRate: 1,
  });
  const journal = new TelemetryJournal({
    client,
    errorJournalPath: errors,
    spanJournalPath: spans,
    cursorPath: path.join(directory, 'cursors.json'),
    log: () => {},
  });

  journal.poll();
  assert.equal(fs.existsSync(queue), false);
  client.update(true);
  fs.appendFileSync(errors, `${JSON.stringify({
    time: '2026-08-29T12:01:00Z',
    stage: 'record',
    class: 'record:demo',
    message: 'C:\\Users\\Luis\\new-secret.dem token=def',
  })}\n`);
  journal.poll();

  const queued = fs.readFileSync(queue, 'utf8');
  assert.match(queued, /pipeline\.error/);
  assert.match(queued, /record:demo/);
  assert.doesNotMatch(queued, /Users|secret\.dem|token|7656119/);
  let events = JSON.parse(queued).events;
  assert.equal(events.length, 1);

  fs.renameSync(errors, `${errors}.1`);
  fs.writeFileSync(errors, `${JSON.stringify({
    time: '2026-08-29T12:02:00Z',
    stage: 'parse',
    class: 'parse:demo',
    message: 'must stay local',
  })}\n`);
  journal.poll();
  events = JSON.parse(fs.readFileSync(queue, 'utf8')).events;
  assert.equal(events.length, 2);
  assert.equal(events[1].stage, 'parse');
  assert.doesNotMatch(JSON.stringify(events), /must stay local/);

  fs.appendFileSync(errors, `${JSON.stringify({
    time: '2026-08-29T12:03:00Z',
    stage: 'player123',
    class: 'LuisPlayer',
    message: 'arbitrary local labels must not cross',
  })}\n`);
  journal.poll();
  events = JSON.parse(fs.readFileSync(queue, 'utf8')).events;
  assert.equal(events.length, 3);
  assert.equal(events[2].stage, 'unknown');
  assert.equal(events[2].class, 'unknown');
  assert.doesNotMatch(JSON.stringify(events), /player123|LuisPlayer|arbitrary local/);

  fs.appendFileSync(spans, `${JSON.stringify({
    time: '2026-08-29T12:04:00Z',
    stage: 'worker',
    name: 'compose:final',
    result: 'ok',
    duration_ms: 250,
  })}\n`);
  fs.renameSync(spans, `${spans}.1`);
  fs.writeFileSync(spans, `${JSON.stringify({
    time: '2026-08-29T12:05:00Z',
    stage: 'worker',
    name: 'render:variant',
    result: 'ok',
    duration_ms: 500,
  })}\n`);
  fs.renameSync(`${spans}.1`, `${spans}.2`);
  fs.renameSync(spans, `${spans}.1`);
  fs.writeFileSync(spans, `${JSON.stringify({
    time: '2026-08-29T12:06:00Z',
    stage: 'worker',
    name: 'record:demo',
    result: 'ok',
    duration_ms: 750,
  })}\n`);
  journal.poll();
  events = JSON.parse(fs.readFileSync(queue, 'utf8')).events;
  assert.equal(events.length, 6);
  assert.deepEqual(
    events.slice(3).map((event: { name: string }) => event.name),
    ['compose:final', 'render:variant', 'record:demo'],
  );
});
