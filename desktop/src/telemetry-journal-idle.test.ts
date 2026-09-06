import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { TelemetryClient } from './telemetry-client.ts';
import { TelemetryJournal } from './telemetry-journal.ts';
import { TelemetrySettingsStore } from './telemetry-settings.ts';

for (const enabled of [false, true]) {
  test(`idle journal does not rewrite cursors (enabled=${enabled})`, (t) => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-journal-idle-'));
    t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
    const errors = path.join(directory, 'errors.jsonl');
    const spans = path.join(directory, 'spans.jsonl');
    const cursors = path.join(directory, 'cursors.json');
    fs.writeFileSync(errors, '');
    fs.writeFileSync(spans, '');
    const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
    const client = new TelemetryClient({
      settings, queuePath: path.join(directory, 'queue.json'), release: '2.4.57',
      config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'test' },
      log: () => {},
    });
    if (enabled) client.update(true);
    const logs: string[] = [];
    const journal = new TelemetryJournal({
      client, errorJournalPath: errors, spanJournalPath: spans, cursorPath: cursors,
      log: (line) => logs.push(line),
    });
    journal.poll();
    fs.utimesSync(cursors, new Date(0), new Date(0));
    const initial = fs.statSync(cursors);
    for (let i = 0; i < 5; i++) journal.poll();
    assert.equal(fs.statSync(cursors).mtimeMs, initial.mtimeMs);
    assert.equal(fs.statSync(cursors).ino, initial.ino);

    fs.appendFileSync(errors, 'ignored line\n');
    journal.poll();
    assert.equal(JSON.parse(fs.readFileSync(cursors, 'utf8')).errors.offset, fs.statSync(errors).size);
    assert.notEqual(fs.statSync(cursors).mtimeMs, initial.mtimeMs);

    // A failed write must not mark changed cursors as successfully persisted.
    fs.rmSync(cursors);
    fs.mkdirSync(cursors);
    fs.appendFileSync(spans, 'another ignored line\n');
    journal.poll();
    assert.equal(logs.length, 1);
    fs.rmdirSync(cursors);
    journal.poll();
    assert.equal(JSON.parse(fs.readFileSync(cursors, 'utf8')).spans.offset, fs.statSync(spans).size);
  });
}
