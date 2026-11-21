import { test } from 'node:test';
import assert from 'node:assert/strict';
import { autoShortsFiles, loteShortVideo, parseShortsBatch, shortsBatchUrl } from './auto-shorts.ts';
import { buildEditRequest } from './edit-request.ts';
import { parseEffectiveEditConfig } from './render-hydration.ts';
import { recoveredVideo } from './recovered-renders.ts';
import { reelIdentity } from './reel-identity.ts';
import { DEFAULT_EDIT_CONFIG } from './reel-store.ts';

const JOB = '29b58ffc-4722-4c77-86c2-d5febb3f9960';
const REVISION = 'ca6e4530-835a-45fc-86da-b3c210a74dec';
const PREFIX = `jobs/${JOB}/renders/viral-60-clean/revisions/${REVISION}`;

test('the batch answer keeps only well-formed segment ids', () => {
  assert.deepEqual(parseShortsBatch({ min_score: 0.6, limit: 5, segment_ids: ['seg-002', 7, '../x', 'seg-009'] }), { minScore: 0.6, limit: 5, segmentIds: ['seg-002', 'seg-009'] });
  assert.equal(parseShortsBatch({ min_score: '0.6', limit: 5, segment_ids: [] }), null);
  assert.equal(parseShortsBatch({ min_score: 0.6, limit: 5 }), null);
  assert.equal(shortsBatchUrl(JOB, { minScore: 0.7, limit: 3 }), `/api/demos/${JOB}/shorts-batch?min_score=0.7&limit=3`);
});

test('a ready batch render lists every Short pinned to its revision', () => {
  const state = { status: 'ready', videos: ['short-002-seg-002.mp4', 'short-005-seg-005.mp4', 'bad name'], artifact_prefix: PREFIX };
  const got = autoShortsFiles(state, { jobId: JOB, variant: 'viral-60-clean', artifactPrefix: PREFIX });
  assert.equal(got.replaced, false);
  assert.deepEqual(got.files.map((file) => file.url), [
    `/api/demos/${JOB}/renders/viral-60-clean/videos/short-002-seg-002.mp4?revision=${REVISION}`,
    `/api/demos/${JOB}/renders/viral-60-clean/videos/short-005-seg-005.mp4?revision=${REVISION}`,
  ]);
  assert.deepEqual(autoShortsFiles({ ...state, status: 'rendering' }, { jobId: JOB, variant: 'viral-60-clean' }), { files: [], replaced: false });
  const other = autoShortsFiles({ ...state, artifact_prefix: `jobs/${JOB}/renders/viral-60-clean/revisions/other` }, { jobId: JOB, variant: 'viral-60-clean', artifactPrefix: PREFIX });
  assert.deepEqual(other, { files: [], replaced: true });
});

test('separate Shorts travel on the edit, come back from the render and get their own reel id', () => {
  const edit = { ...DEFAULT_EDIT_CONFIG, separateShorts: true };
  const body = buildEditRequest(edit);
  assert.equal(body.separate_shorts, true);
  assert.equal(buildEditRequest({ ...edit, deathReel: true }).separate_shorts, undefined);
  assert.equal(parseEffectiveEditConfig(body)?.separateShorts, true);
  assert.equal(reelIdentity({ matchId: JOB, playIds: ['seg-002', 'seg-005'], editConfig: edit }), `${JOB}__auto__seg-002_seg-005`);
  assert.equal(reelIdentity({ matchId: JOB, playIds: ['seg-002', 'seg-005'], editConfig: DEFAULT_EDIT_CONFIG }), `${JOB}__seg-002_seg-005`);
});

test('a batch render found on disk is recovered under the batch reel id', () => {
  const job = { jobId: JOB, status: 'recorded', createdAt: '2026-09-14T13:50:29Z' };
  const render = { status: 'ready', videoName: 'short-002-seg-002.mp4', segmentIds: ['seg-002', 'seg-005'], editConfig: { ...DEFAULT_EDIT_CONFIG, separateShorts: true } };
  const found = recoveredVideo(job, 'viral-60-clean', render, new Set());
  assert.equal(found?.video.id, `${JOB}__auto__seg-002_seg-005`);
  assert.equal(found?.video.title, 'Auto-Shorts: 2 Shorts - Killfeed');
  assert.equal(recoveredVideo(job, 'viral-60-clean', render, new Set([`${JOB}__auto__seg-002_seg-005`])), null);
});

test('the publishing page of a lote opened on one of its Shorts shows and downloads that Short', () => {
  const lote = {
    id: `${JOB}__auto__seg-001_seg-004`, jobId: JOB, title: 'Auto-Shorts: 2 Shorts - Viral', map: 'de_mirage', score: '', mode: 'clean' as const,
    variant: 'viral-60-clean', status: 'ready' as const, createdAt: 0, editConfig: { ...DEFAULT_EDIT_CONFIG, separateShorts: true },
    downloadUrl: `/api/demos/${JOB}/renders/viral-60-clean/videos/seg-001`, thumbnailUrl: `/api/demos/${JOB}/renders/viral-60-clean/covers/seg-001`,
  };
  const shown = loteShortVideo(lote, 'seg-004');
  assert.equal(shown.downloadUrl, `/api/demos/${JOB}/renders/viral-60-clean/videos/seg-004`);
  assert.equal(shown.thumbnailUrl, undefined);
  assert.equal(loteShortVideo(lote, null), lote);
  assert.equal(loteShortVideo(lote, '../x'), lote);
  const single = { ...lote, editConfig: DEFAULT_EDIT_CONFIG };
  assert.equal(loteShortVideo(single, 'seg-004'), single);
});
