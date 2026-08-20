import { strict as assert } from 'node:assert';
import test, { beforeEach } from 'node:test';
import type { Video } from './api/types.ts';
import {
  publishShellActivity,
  resetShellActivity,
  serverShellActivitySnapshot,
  shellActivityIsStale,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from './shell-activity.ts';

function reel(overrides: Partial<Video> & Pick<Video, 'id' | 'status'>): Video {
  return {
    title: `reel ${overrides.id}`,
    map: 'de_mirage',
    score: '16-14',
    mode: 'clean',
    createdAt: 1000,
    ...overrides,
  };
}

beforeEach(() => {
  resetShellActivity();
});

test('terminal reels never reach the shell', () => {
  publishShellActivity([reel({ id: 'a', status: 'ready' }), reel({ id: 'b', status: 'failed' })], 10);
  const snapshot = shellActivitySnapshot();
  assert.deepEqual(snapshot.jobs, []);
  assert.equal(snapshot.capturing, false);
});

test('a queued reel is activity but not GPU contention', () => {
  publishShellActivity([reel({ id: 'a', status: 'queued' })], 10);
  const snapshot = shellActivitySnapshot();
  assert.equal(snapshot.jobs.length, 1);
  assert.equal(snapshot.capturing, false);
});

test('recording and composing both mark the GPU busy', () => {
  publishShellActivity([reel({ id: 'a', status: 'recording' })], 10);
  assert.equal(shellActivitySnapshot().capturing, true);
  publishShellActivity([reel({ id: 'a', status: 'composing' })], 20);
  assert.equal(shellActivitySnapshot().capturing, true);
});

test('jobs are ordered most-advanced first, then oldest first', () => {
  publishShellActivity(
    [
      reel({ id: 'queued', status: 'queued', createdAt: 1 }),
      reel({ id: 'composing', status: 'composing', createdAt: 2 }),
      reel({ id: 'new-recording', status: 'recording', createdAt: 9 }),
      reel({ id: 'old-recording', status: 'recording', createdAt: 3 }),
    ],
    10,
  );
  assert.deepEqual(
    shellActivitySnapshot().jobs.map((job) => job.id),
    ['old-recording', 'new-recording', 'composing', 'queued'],
  );
});

test('capture progress is carried only where the API reports it', () => {
  publishShellActivity(
    [
      reel({ id: 'a', status: 'recording', captureProgress: { done: 3, total: 8, percent: 41 } }),
      reel({ id: 'b', status: 'composing', captureProgress: { done: 8, total: 8 } }),
      reel({ id: 'c', status: 'queued' }),
    ],
    10,
  );
  const [recording, composing, queued] = shellActivitySnapshot().jobs;
  assert.deepEqual(recording?.progress, { done: 3, total: 8, percent: 41 });
  // Composing has no segment counter of its own; a stale capture count there
  // would be fabricated progress.
  assert.equal(composing?.progress, null);
  assert.equal(queued?.progress, null);
});

test('a live percent change wakes subscribers even when the clip count does not', () => {
  let notifications = 0;
  const unsubscribe = subscribeToShellActivity(() => {
    notifications += 1;
  });

  publishShellActivity(
    [reel({ id: 'a', status: 'recording', captureProgress: { done: 3, total: 4, percent: 75 } })],
    10,
  );
  publishShellActivity(
    [reel({ id: 'a', status: 'recording', captureProgress: { done: 3, total: 4, percent: 82 } })],
    20,
  );
  assert.equal(notifications, 2);
  assert.equal(shellActivitySnapshot().jobs[0]?.progress?.percent, 82);

  unsubscribe();
});

test('a zero-segment capture reports no progress instead of dividing by zero', () => {
  publishShellActivity([reel({ id: 'a', status: 'recording', captureProgress: { done: 0, total: 0 } })], 10);
  assert.equal(shellActivitySnapshot().jobs[0]?.progress, null);
});

test('an unchanged payload refreshes freshness without waking subscribers', () => {
  let notifications = 0;
  const unsubscribe = subscribeToShellActivity(() => {
    notifications += 1;
  });

  publishShellActivity([reel({ id: 'a', status: 'recording' })], 1000);
  assert.equal(notifications, 1);

  publishShellActivity([reel({ id: 'a', status: 'recording' })], 2500);
  assert.equal(notifications, 1);
  assert.equal(shellActivitySnapshot().publishedAt, 2500);

  publishShellActivity([reel({ id: 'a', status: 'composing' })], 3000);
  assert.equal(notifications, 2);

  unsubscribe();
  publishShellActivity([], 4000);
  assert.equal(notifications, 2);
});

test('freshness expires so the shell resumes polling when a page stops pushing', () => {
  publishShellActivity([reel({ id: 'a', status: 'recording' })], 10_000);
  assert.equal(shellActivityIsStale(11_000), false);
  assert.equal(shellActivityIsStale(14_000), true);
});

test('nothing has ever published before the first push', () => {
  assert.equal(shellActivityIsStale(0), true);
  assert.deepEqual(serverShellActivitySnapshot(), shellActivitySnapshot());
});
