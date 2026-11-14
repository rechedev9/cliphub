import assert from 'node:assert/strict';
import test from 'node:test';
import {
  addFiles,
  clearFinished,
  emptyQueue,
  isDemoFileName,
  progress,
  remove,
  resume,
  retry,
  retryAllFailed,
  scanFailed,
  scanSucceeded,
  schedule,
  summarize,
  uploadFailed,
  uploadSucceeded,
  type ItemStatus,
  type QueueState,
} from './import-queue.ts';

function demo(id: string, name = `${id}.dem.zst`) {
  return { id, name, size: 100, lastModified: 1 };
}

function queued(...ids: string[]): QueueState {
  return addFiles(emptyQueue, ids.map((id) => demo(id))).state;
}

function statuses(state: QueueState): Record<string, ItemStatus> {
  return Object.fromEntries(state.items.map((item) => [item.id, item.status]));
}

test('only demo extensions enter the queue; other files and repeats are reported by name', () => {
  assert.ok(isDemoFileName('1-abc-1-2.dem.zst'));
  assert.ok(isDemoFileName('match.DEM'));
  assert.ok(isDemoFileName('old.dem.gz'));
  assert.ok(isDemoFileName('1-abc.dem (1).zst'));
  assert.ok(!isDemoFileName('match.rar'));

  const first = addFiles(emptyQueue, [demo('a'), { ...demo('b'), name: 'notes.txt' }, demo('c')]);
  assert.deepEqual(first.added, ['a', 'c']);
  assert.deepEqual(first.unsupported, ['notes.txt']);

  const again = addFiles(first.state, [{ ...demo('a2'), name: 'a.dem.zst' }, demo('d')]);
  assert.deepEqual(again.added, ['d']);
  assert.deepEqual(again.repeated, ['a.dem.zst']);
  assert.deepEqual(
    again.state.items.map((item) => item.id),
    ['a', 'c', 'd'],
  );
});

test('scans one demo at a time, in order, and uploads the previous one meanwhile', () => {
  let plan = schedule(queued('a', 'b', 'c'));
  assert.equal(plan.scan, 'a');
  assert.equal(plan.upload, undefined);
  assert.deepEqual(statuses(plan.state), { a: 'scanning', b: 'queued', c: 'queued' });
  assert.equal(schedule(plan.state).scan, undefined, 'a second scan never starts while one runs');

  plan = schedule(scanSucceeded(plan.state, { id: 'a', sha: 'sha-a', map: 'de_nuke' }));
  assert.equal(plan.upload, 'a');
  assert.equal(plan.scan, 'b');
  assert.deepEqual(statuses(plan.state), { a: 'uploading', b: 'scanning', c: 'queued' });
});

test('a scan waits while one analysis already waits behind a slow upload', () => {
  let plan = schedule(queued('a', 'b', 'c'));
  plan = schedule(scanSucceeded(plan.state, { id: 'a', sha: 'sha-a', map: 'de_nuke' }));
  plan = schedule(scanSucceeded(plan.state, { id: 'b', sha: 'sha-b', map: 'de_nuke' }));
  assert.equal(plan.scan, undefined);
  assert.deepEqual(statuses(plan.state), { a: 'uploading', b: 'scanned', c: 'queued' });

  plan = schedule(uploadSucceeded(plan.state, { id: 'a', matchId: 'm-a', created: true }));
  assert.equal(plan.upload, 'b');
  assert.equal(plan.scan, 'c');
});

test('a failed file does not stop the others', () => {
  let plan = schedule(queued('a', 'b'));
  plan = schedule(scanFailed(plan.state, { id: 'a', error: 'Este archivo no es una demo de CS2.' }));
  assert.equal(plan.scan, 'b');
  plan = schedule(scanSucceeded(plan.state, { id: 'b', sha: 'sha-b', map: 'de_inferno' }));
  plan = schedule(uploadFailed(plan.state, { id: 'b', error: 'ClipMind no pudo guardar esta partida.', detail: 'map has no calibration' }));
  assert.deepEqual(statuses(plan.state), { a: 'failed', b: 'failed' });
  assert.equal(plan.state.items[1]?.detail, 'map has no calibration');
  assert.deepEqual(summarize(plan.state), { total: 2, imported: 0, duplicate: 0, failed: 2, pending: 0, finished: 2 });
});

test('retry reads the demo again after a failed scan and only re-sends after a failed upload', () => {
  let plan = schedule(queued('a', 'b'));
  plan = schedule(scanFailed(plan.state, { id: 'a', error: 'x' }));
  plan = schedule(scanSucceeded(plan.state, { id: 'b', sha: 'sha-b', map: 'de_mirage' }));
  plan = schedule(uploadFailed(plan.state, { id: 'b', error: 'y' }));

  const retried = retryAllFailed(plan.state);
  assert.deepEqual(statuses(retried), { a: 'queued', b: 'scanned' });
  assert.equal(retried.items[0]?.error, undefined);
  assert.equal(retried.items[1]?.sha, 'sha-b');

  const next = schedule(retried);
  assert.equal(next.upload, 'b');
  assert.equal(next.scan, 'a');
  assert.equal(retry(next.state, 'a'), next.state, 'only a failed item retries');
});

test('the server answer or an earlier twin in this session marks a demo as already in the library', () => {
  let plan = schedule(queued('a', 'b', 'c'));
  plan = schedule(scanSucceeded(plan.state, { id: 'a', sha: 'same', map: 'de_ancient' }));
  plan = schedule(uploadSucceeded(plan.state, { id: 'a', matchId: 'm-1', created: false }));
  assert.equal(plan.state.items[0]?.status, 'duplicate');

  // b is a.dem.zst's plain .dem: same checksum, so it skips the upload.
  plan = schedule(scanSucceeded(plan.state, { id: 'b', sha: 'same', map: 'de_ancient' }));
  assert.equal(plan.upload, undefined);
  assert.equal(plan.scan, 'c');
  assert.equal(plan.state.items[1]?.status, 'duplicate');
  assert.equal(plan.state.items[1]?.matchId, 'm-1');
});

test('a lost connection pauses the queue and keeps the analysis for when it resumes', () => {
  let plan = schedule(queued('a', 'b', 'c'));
  plan = schedule(scanSucceeded(plan.state, { id: 'a', sha: 'sha-a', map: 'de_dust2' }));
  plan = schedule(uploadFailed(plan.state, { id: 'a', error: 'Sin conexión con ClipMind.', pause: 'offline' }));
  assert.equal(plan.state.paused, 'offline');
  assert.deepEqual(statuses(plan.state), { a: 'scanned', b: 'scanning', c: 'queued' });

  plan = schedule(scanSucceeded(plan.state, { id: 'b', sha: 'sha-b', map: 'de_dust2' }));
  assert.equal(plan.upload, undefined, 'nothing uploads while paused');
  assert.equal(plan.scan, undefined);

  plan = schedule(resume(plan.state));
  assert.equal(plan.state.paused, undefined);
  assert.equal(plan.upload, 'a');
  assert.equal(plan.scan, undefined, 'b still waits behind a');
});

test('progress moves by whole percents and only for work in flight', () => {
  const plan = schedule(queued('a', 'b'));
  const ticked = progress(plan.state, { id: 'a', done: 425, total: 1000 });
  assert.equal(ticked.items[0]?.percent, 42);
  assert.equal(progress(ticked, { id: 'a', done: 429, total: 1000 }), ticked, 'same whole percent, same state');
  assert.equal(progress(ticked, { id: 'b', done: 1, total: 2 }), ticked, 'a queued demo has no progress');
  assert.equal(progress(ticked, { id: 'a', done: 5, total: 0 }).items[0]?.percent, 0);
});

test('remove and clearFinished drop rows; the queue moves on without them', () => {
  let plan = schedule(queued('a', 'b', 'c'));
  plan = schedule(remove(plan.state, 'a'));
  assert.equal(plan.scan, 'b');
  plan = schedule(scanSucceeded(plan.state, { id: 'b', sha: 'sha-b', map: 'de_train' }));
  plan = schedule(uploadSucceeded(plan.state, { id: 'b', matchId: 'm-b', created: true }));
  assert.deepEqual(
    clearFinished(plan.state).items.map((item) => item.id),
    ['c'],
  );
  assert.equal(remove(plan.state, 'missing'), plan.state);
});
