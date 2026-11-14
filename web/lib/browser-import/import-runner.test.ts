import assert from 'node:assert/strict';
import test from 'node:test';
import { describeScanFailure, ImportRunner, type ScanOptions } from './import-runner.ts';
import type { ScanResult } from './scan-messages.ts';
import type { UploadInput, UploadOutcome } from './upload-analysis.ts';

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
}

function deferred<T>(): Deferred<T> {
  let resolve: (value: T) => void = () => undefined;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function scanned(name: string): ScanResult {
  return {
    ok: true,
    sha256: `sha-${name}`,
    map: 'de_mirage',
    rounds: 24,
    cs2Build: 1,
    document: '{}',
    positions: new Uint8Array([1]),
    sizeBytes: 10,
    fileName: name.replace(/\.zst$/, ''),
  };
}

const flush = (): Promise<void> => new Promise((done) => setTimeout(done, 0));

function harness() {
  const scans = new Map<string, Deferred<ScanResult>>();
  const uploads = new Map<string, Deferred<UploadOutcome>>();
  const scanOptions = new Map<string, ScanOptions>();
  const uploaded: UploadInput[] = [];
  let next = 0;
  const runner = new ImportRunner({
    newId: () => `id-${(next += 1)}`,
    scan: (file, options) => {
      const pending = deferred<ScanResult>();
      scans.set(file.name, pending);
      scanOptions.set(file.name, options);
      return pending.promise;
    },
    upload: (input) => {
      const pending = deferred<UploadOutcome>();
      uploads.set(input.scan.fileName, pending);
      uploaded.push(input);
      return pending.promise;
    },
    progressEveryMs: 1,
  });
  return { runner, scans, uploads, scanOptions, uploaded };
}

function file(name: string): File {
  return new File([new Uint8Array(4)], name, { lastModified: 1 });
}

function statusOf(runner: ImportRunner): string[] {
  return runner.getSnapshot().items.map((item) => `${item.name}:${item.status}`);
}

test('the runner overlaps the next scan with the previous upload and keeps going after a failure', async () => {
  const { runner, scans, uploads, uploaded } = harness();
  const report = runner.add([file('a.dem.zst'), file('b.dem'), file('c.dem.gz'), file('notes.txt')]);
  assert.deepEqual(report, { added: 3, unsupported: ['notes.txt'], repeated: [] });
  assert.deepEqual([...scans.keys()], ['a.dem.zst'], 'one scan at a time');

  scans.get('a.dem.zst')?.resolve(scanned('a.dem.zst'));
  await flush();
  assert.deepEqual(statusOf(runner), ['a.dem.zst:uploading', 'b.dem:scanning', 'c.dem.gz:queued']);
  assert.equal(uploaded[0]?.scan.fileName, 'a.dem');

  scans.get('b.dem')?.resolve({ ok: false, error: 'parsing demo: unexpected end' });
  await flush();
  assert.deepEqual(statusOf(runner), ['a.dem.zst:uploading', 'b.dem:failed', 'c.dem.gz:scanning']);

  uploads.get('a.dem')?.resolve({ ok: true, matchId: 'm-a', created: true });
  scans.get('c.dem.gz')?.resolve(scanned('c.dem.gz'));
  await flush();
  uploads.get('c.dem.gz')?.resolve({ ok: true, matchId: 'm-c', created: false });
  await flush();
  assert.deepEqual(statusOf(runner), ['a.dem.zst:imported', 'b.dem:failed', 'c.dem.gz:duplicate']);
  assert.equal(runner.getSnapshot().items[1]?.detail, 'parsing demo: unexpected end');
});

test('progress repaints are throttled and a removed demo stops its scan', async () => {
  const { runner, scans, scanOptions } = harness();
  let paints = 0;
  runner.subscribe(() => {
    paints += 1;
  });
  runner.add([file('a.dem'), file('b.dem')]);
  const before = paints;
  const options = scanOptions.get('a.dem');
  for (let read = 1; read <= 50; read += 1) options?.onProgress(read, 100);
  assert.equal(paints, before, 'ticks wait for the interval');
  await new Promise((done) => setTimeout(done, 5));
  assert.equal(paints, before + 1);
  assert.equal(runner.getSnapshot().items[0]?.percent, 50);

  const id = runner.getSnapshot().items[0]?.id ?? '';
  runner.remove(id);
  assert.equal(options?.signal.aborted, true);
  assert.deepEqual([...scans.keys()], ['a.dem', 'b.dem'], 'the next demo starts at once');
});

test('scanner sentences reach the coach as they are; anything else gets a calm summary', () => {
  assert.deepEqual(describeScanFailure('es una demo de CS:GO; solo se admiten demos de CS2'), {
    error: 'Es una demo de CS:GO; solo se admiten demos de CS2.',
  });
  assert.deepEqual(describeScanFailure('parsing demo: bad packet'), {
    error: 'No se pudo analizar la demo. Puede estar dañada o no ser de CS2.',
    detail: 'parsing demo: bad packet',
  });
});
