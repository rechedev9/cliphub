import test from 'node:test';
import assert from 'node:assert/strict';
import { importedVideo, importedVideoId, isImportedVideo, parseImportedVideos } from './imported-videos.ts';
import type { IndexedJob } from './jobs-index.ts';

const ID = '11111111-1111-4111-8111-111111111111';

test('parseImportedVideos keeps well-formed rows and drops the rest', () => {
  const parsed = parseImportedVideos({
    videos: [
      { cloud_job_id: ID, name: 'short-01.mp4', title: '3 jugadas - Kill Feed', local_job_id: 'job-a', created_at: '2026-10-20T10:00:00Z' },
      { cloud_job_id: ID, name: 'short-02.mp4', title: 'Sin fecha', local_job_id: '' },
      { cloud_job_id: ID, title: 'no name' },
      'junk',
    ],
  });
  assert.deepEqual(parsed, [
    { cloudJobId: ID, name: 'short-01.mp4', title: '3 jugadas - Kill Feed', localJobId: 'job-a', createdAt: Date.parse('2026-10-20T10:00:00Z') },
    { cloudJobId: ID, name: 'short-02.mp4', title: 'Sin fecha', createdAt: 0 },
  ]);
  assert.deepEqual(parseImportedVideos({ videos: null }), []);
  assert.deepEqual(parseImportedVideos(null), []);
});

test('an imported Short is a ready read-only video that takes the map of its demo while it exists', () => {
  const entry = { cloudJobId: ID, name: 'short-01.mp4', title: 'Short', localJobId: 'job-a', createdAt: 5 };
  const jobs: IndexedJob[] = [{ jobId: 'job-a', status: 'parsed', summary: { match: { map: 'de_mirage' }, target: { steamid64: '7656', name: 'donk' } } }];
  const video = importedVideo(entry, '/api/imported-videos/x/short-01.mp4', jobs);
  assert.equal(video.id, importedVideoId(entry));
  assert.equal(isImportedVideo(video), true);
  assert.equal(isImportedVideo({ id: 'job-a__viral-60-clean' }), false);
  assert.equal(video.status, 'ready');
  assert.equal(video.recovered, true);
  assert.equal(video.publishable, false);
  assert.equal(video.jobId, undefined);
  assert.equal(video.map, 'de_mirage');
  assert.equal(video.targetName, 'donk');
  assert.equal(video.editConfig?.format, 'short-9x16');
  assert.equal(video.downloadUrl, '/api/imported-videos/x/short-01.mp4');
  assert.equal(importedVideo(entry, '', []).map, 'Unknown');
});
