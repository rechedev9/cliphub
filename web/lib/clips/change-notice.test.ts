import test from 'node:test';
import assert from 'node:assert/strict';
import { CHANGE_NOT_APPLIED_REASON, DEMO_INCOMPATIBLE_PREFIX, parseFailureReason } from '../api/failure-reason.ts';
import type { Video } from '../api/types.ts';
import { changeNotice } from './change-notice.ts';

const DRIFT = 'recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001';

function reel(overrides: Partial<Video>): Video {
  return { id: 'job-1__seg-001', title: 'Clip', map: 'de_mirage', score: '13-9', mode: 'clean', status: 'ready', createdAt: 0, ...overrides };
}

test('a video says why its last change is missing only while a change is pending', () => {
  assert.equal(changeNotice(reel({})), null, 'a ready video with nothing pending says nothing');
  assert.equal(changeNotice(reel({ status: 'failed', failureReason: DRIFT })), null, 'a failed video shows its own failure');

  assert.deepEqual(
    changeNotice(reel({ changePending: true })),
    { message: CHANGE_NOT_APPLIED_REASON, retryCanHelp: true },
    'a change no capture was tried for waits for the click',
  );

  const failed = changeNotice(reel({ changePending: true, failureReason: DRIFT }));
  assert.ok(failed, 'a failed capture is told');
  assert.ok(failed.message.endsWith(parseFailureReason(DRIFT).message), 'a failed capture names its cause');
  assert.ok(!failed.message.includes(CHANGE_NOT_APPLIED_REASON), 'a failed capture is not told as a change still waiting');
  assert.equal(failed.retryCanHelp, true);

  const hopeless = changeNotice(reel({ changePending: true, failureReason: `${DEMO_INCOMPATIBLE_PREFIX} old build` }));
  assert.equal(hopeless?.retryCanHelp, false, 'a cause a retry cannot fix offers no retry');
});
