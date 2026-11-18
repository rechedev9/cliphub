import { test } from 'node:test';
import assert from 'node:assert/strict';
import { recoveredVideo, recoveredVideoId, recoveryProbes, RECOVERY_FULL_DEMO_VARIANT, RECOVERY_VARIANTS } from './recovered-renders.ts';
import type { IndexedJob } from './jobs-index.ts';
import { reelIdentity } from './reel-identity.ts';
import { DEFAULT_EDIT_CONFIG } from './reel-store.ts';
import type { EditConfig } from './types.ts';

const JOB = '29b58ffc-4722-4c77-86c2-d5febb3f9960';
const REVISION = 'ca6e4530-835a-45fc-86da-b3c210a74dec';
const job: IndexedJob = {
  jobId: JOB,
  status: 'recorded',
  createdAt: '2026-09-14T13:50:29Z',
  summary: { match: { map: 'de_ancient' }, target: { steamid64: '76561198305036904', name: 'Ahogaq' } },
};

test('a delivered Full Demo with no intent becomes a ready long video pinned to its revision', () => {
  const found = recoveredVideo(job, RECOVERY_FULL_DEMO_VARIANT, {
    status: 'ready',
    videoName: 'demo-compilation.mp4',
    artifactPrefix: `jobs/${JOB}/renders/gameplay-pov-60/revisions/${REVISION}`,
  }, new Set());
  assert.ok(found);
  assert.equal(found.video.id, `${JOB}__full-demo`);
  assert.equal(found.video.status, 'ready');
  assert.equal(found.video.editConfig?.format, 'landscape-16x9');
  assert.equal(found.video.editConfig?.matchRecap, true);
  assert.equal(found.video.map, 'de_ancient');
  assert.equal(found.video.targetName, 'Ahogaq');
  assert.equal(found.video.downloadUrl, `/api/demos/${JOB}/renders/gameplay-pov-60/videos/demo-compilation.mp4?revision=${REVISION}`);
  assert.equal(found.video.createdAt, Date.parse('2026-09-14T13:50:29Z'));
});

test('renders an intent still owns, unfinished renders and renders without an MP4 are not recovered', () => {
  const ready = { status: 'ready', videoName: 'demo-compilation.mp4' };
  assert.equal(recoveredVideo(job, RECOVERY_FULL_DEMO_VARIANT, ready, new Set([`${JOB}__full-demo`])), null);
  assert.equal(recoveredVideo(job, RECOVERY_FULL_DEMO_VARIANT, { status: 'rendering' }, new Set()), null);
  assert.equal(recoveredVideo(job, RECOVERY_FULL_DEMO_VARIANT, { status: 'ready' }, new Set()), null);
});

test('a recovered Short keeps its segment identity and a vertical format', () => {
  const found = recoveredVideo(job, 'viral-60-clean', { status: 'ready', videoName: 'short.mp4', segmentIds: ['seg-001', 'seg-004'] }, new Set());
  assert.ok(found);
  assert.equal(found.video.id, `${JOB}__seg-001_seg-004`);
  assert.equal(found.video.editConfig?.format, 'short-9x16');
  assert.equal(found.video.title, '2 jugadas - Kill Feed');
  assert.equal(recoveredVideoId(JOB, 'viral-60-clean', []), null);
});

test('a delivered Crear horizontal is recovered under its own reel, never as Demo completa', () => {
  const edit: EditConfig = { ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9', captureHud: 'deathnotices', portraitSafeKillfeed: true };
  const render = {
    status: 'ready', videoName: 'summary.mp4', segmentIds: ['seg-002', 'seg-003'], editConfig: edit,
    artifactPrefix: `jobs/${JOB}/renders/gameplay-pov-60/revisions/${REVISION}`,
  };
  const id = reelIdentity({ matchId: JOB, playIds: ['seg-002', 'seg-003'], editConfig: edit });
  assert.equal(recoveredVideo(job, RECOVERY_FULL_DEMO_VARIANT, render, new Set([id])), null, 'its own reel still owns it');
  const found = recoveredVideo(job, RECOVERY_FULL_DEMO_VARIANT, render, new Set());
  assert.ok(found);
  assert.equal(found.video.id, id);
  assert.equal(found.video.editConfig?.format, 'landscape-16x9');
  assert.equal(found.video.editConfig?.matchRecap, false);
  assert.equal(found.video.title, '2 jugadas - POV nativo');
  assert.equal(found.video.downloadUrl, `/api/demos/${JOB}/renders/gameplay-pov-60/videos/summary.mp4?revision=${REVISION}`);
});

test('jobs are probed once per status and only when they can hold render state', () => {
  const jobs: IndexedJob[] = [job, { jobId: 'a156e14f-6fa7-4e43-b45e-4bc84ade62f2', status: 'parsed' }];
  assert.equal(recoveryProbes(jobs, new Map()).length, RECOVERY_VARIANTS.length);
  assert.deepEqual(recoveryProbes(jobs, new Map([[JOB, 'recorded']])), []);
  assert.equal(recoveryProbes(jobs, new Map([[JOB, 'failed']])).length, RECOVERY_VARIANTS.length);
});
