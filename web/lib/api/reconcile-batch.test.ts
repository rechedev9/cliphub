import assert from 'node:assert/strict';
import test from 'node:test';

import { reconcileReels } from './reconcile-batch.ts';
import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

function serviceUnavailable(): Error & { code: string } {
  return Object.assign(
    new Error('analysis service unavailable'),
    { code: SERVICE_UNAVAILABLE_CODE },
  );
}

test('one failed reel does not hide another successful cached reel', async () => {
  const visible = new Map<string, string>([
    ['review-reel', 'review_required'],
    ['cached-reel', 'queued'],
  ]);
  const reelFailure = Promise.reject(serviceUnavailable());
  const cachedSuccess = Promise.resolve().then(() => {
    visible.set('cached-reel', 'ready');
  });

  await reconcileReels([reelFailure, cachedSuccess]);

  assert.deepEqual(
    Array.from(visible.entries()),
    [
      ['review-reel', 'review_required'],
      ['cached-reel', 'ready'],
    ],
  );
});

test('a genuinely global service outage still rejects the refresh', async () => {
  await assert.rejects(
    reconcileReels([
      Promise.reject(serviceUnavailable()),
      Promise.reject(serviceUnavailable()),
    ]),
    (error: unknown) => error instanceof Error
      && 'code' in error
      && error.code === SERVICE_UNAVAILABLE_CODE,
  );
});
