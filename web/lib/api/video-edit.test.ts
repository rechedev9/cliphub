import test from 'node:test';
import assert from 'node:assert/strict';
import { projectCaptureCheck, projectSpace, projectVideoVersions } from './video-edit.ts';

const REV_A = '11111111-1111-4111-8111-111111111111';
const REV_B = '22222222-2222-4222-8222-222222222222';
const PLAN = '33333333-3333-4333-8333-333333333333';

test('projectVideoVersions keeps well-formed versions in the server order', () => {
  const body = {
    versions: [
      { revision: REV_B, created_at: '2026-11-18T10:00:00Z', current: true, videos: ['full-demo'], edit: { format: 'landscape-16x9' }, plan_id: PLAN, rounds: [3, 5] },
      { revision: REV_A, created_at: '2026-11-17T10:00:00Z', current: false, videos: ['seg-001', '../x'], segment_ids: ['seg-001'], edit: {}, music: { key: 'phonk-01', volume: 0.5 } },
      { revision: 'not-a-uuid', created_at: '2026-11-16T10:00:00Z', current: false, videos: ['a'] },
      null,
    ],
  };
  assert.deepEqual(projectVideoVersions(body), [
    { revision: REV_B, createdAt: '2026-11-18T10:00:00Z', current: true, videos: ['full-demo'], segmentIds: [], edit: { format: 'landscape-16x9' }, music: undefined, planId: PLAN, rounds: [3, 5] },
    { revision: REV_A, createdAt: '2026-11-17T10:00:00Z', current: false, videos: ['seg-001'], segmentIds: ['seg-001'], edit: {}, music: { key: 'phonk-01', volume: 0.5 }, planId: null, rounds: [] },
  ]);
  assert.deepEqual(projectVideoVersions({}), []);
});

test('projectCaptureCheck reads the capture need and the space estimate', () => {
  assert.deepEqual(projectCaptureCheck({ needed: true, missing_rounds: [4, 2], space: { free_bytes: 10, needed_bytes: 20, short: true } }), {
    needed: true,
    missingRounds: [4, 2],
    space: { freeBytes: 10, neededBytes: 20, short: true },
  });
  assert.deepEqual(projectCaptureCheck({ needed: false, missing_rounds: [] }), { needed: false, missingRounds: [], space: null });
  assert.equal(projectCaptureCheck({ needed: 'yes' }), null);
  assert.equal(projectSpace({ free_bytes: -1, needed_bytes: 2, short: true }), null);
});
