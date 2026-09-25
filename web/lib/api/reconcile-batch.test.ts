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
  await assert.doesNotReject(reconcileReels([Promise.reject(serviceUnavailable()), Promise.resolve()]));
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
