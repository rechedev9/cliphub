import assert from 'node:assert/strict';
import test from 'node:test';
import { demoPlaybackItem, PLAYBACK_REVIEW, streamPlaybackItem } from './playback.ts';
import type { StreamJob, StreamRenderedOutput } from './streams.ts';
import type { Video } from './types.ts';

test('demo playback identity includes its immutable artifact revision', () => {
  const video: Video = {
    id: 'reel-1',
    jobId: 'job-1',
    title: 'Ace',
    map: 'de_mirage',
    score: '13-8',
    mode: 'clean',
    variant: 'viral-60-clean',
    status: 'review_required',
    createdAt: 1,
    downloadUrl: '/api/demos/job-1/renders/viral-60-clean/videos/ace.mp4',
    reviewArtifactPrefix: 'jobs/job-1/renders/viral-60-clean/revisions/123e4567-e89b-42d3-a456-426614174000',
    warnings: ['freeze at 00:12'],
  };

  const item = demoPlaybackItem(video);
  assert.equal(item?.revision, '123e4567-e89b-42d3-a456-426614174000');
  assert.equal(item?.revision.includes('jobs/'), false);
  assert.equal(item?.playbackUrl, '/api/demos/job-1/renders/viral-60-clean/revisions/123e4567-e89b-42d3-a456-426614174000/videos/ace.mp4');
  assert.equal(item?.review, PLAYBACK_REVIEW.pending);
  assert.deepEqual(item?.warnings, video.warnings);
});

test('stream playback uses the revision URL and published review state', () => {
  const job: StreamJob = { id: 'job-2', status: 'rendered', title: 'Final', created_at: '2026-09-05T10:00:00Z' };
  const output: StreamRenderedOutput = {
    artifact_revision: 'revision-3',
    variant: 'streamer-fullframe-nocam',
    clip_id: 'clip-1',
    artifact_name: 'clip-1.mp4',
    format: 'video/mp4',
    aspect_ratio: '9:16',
    video_url: '/api/streams/job-2/renders/streamer-fullframe-nocam/revisions/revision-3/videos/clip-1',
    render_status: 'rendered',
    stale: true,
    review_required: true,
    warnings: ['dead air'],
  };

  const item = streamPlaybackItem(job, output);
  assert.equal(item?.playbackUrl, output.video_url);
  assert.equal(item?.revision, output.artifact_revision);
  assert.equal(item?.review, PLAYBACK_REVIEW.stale);
  assert.deepEqual(item?.warnings, ['dead air']);
});

test('stream playback rejects unfinished and cross-site output URLs', () => {
  const job: StreamJob = { id: 'job-3', status: 'rendered', created_at: '2026-09-05T10:00:00Z' };
  const base: StreamRenderedOutput = {
    artifact_revision: 'revision-4',
    variant: 'streamer-fullframe-nocam',
    clip_id: 'clip-1',
    artifact_name: 'clip-1.mp4',
    format: 'video/mp4',
    aspect_ratio: '9:16',
    video_url: 'https://example.com/video.mp4',
    render_status: 'rendered',
    stale: false,
    review_required: false,
  };

  assert.equal(streamPlaybackItem(job, base), null);
  assert.equal(streamPlaybackItem(job, { ...base, video_url: '/api/streams/job-3/video', render_status: 'rendering' }), null);
});

test('demo playback rejects malformed artifact names instead of breaking the library', () => {
  const video: Video = {
    id: 'bad', jobId: 'job', title: 'Bad URL', map: 'de_nuke', score: '', mode: 'clean', status: 'ready',
    createdAt: 1, downloadUrl: '/api/demos/job/videos/%',
  };
  assert.equal(demoPlaybackItem(video), null);
});

test('legacy demo outputs remain on their mutable route with an explicit legacy identity', () => {
  const video: Video = {
    id: 'old', jobId: 'job', title: 'Legacy', map: 'de_nuke', score: '', mode: 'clean', status: 'ready',
    createdAt: 1, downloadUrl: '/api/demos/job/renders/clean/videos/old.mp4',
    reviewArtifactPrefix: 'jobs/job/renders/clean/revisions/old-pointer',
  };
  const item = demoPlaybackItem(video);
  assert.equal(item?.playbackUrl, video.downloadUrl);
  assert.match(item?.revision ?? '', /^legacy:/);
  assert.equal(item?.revision.includes('jobs/'), false);
});
