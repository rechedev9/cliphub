import test from 'node:test';
import assert from 'node:assert/strict';
import { publicEditorAsset, publicStreamJob } from './public-projections.ts';

test('stream job projection drops local filesystem facts and keeps the Studio contract', () => {
  const raw = {
    id: 'a',
    status: 'ready',
    failure_reason: 'x',
    failure_code: 'acquire_failed',
    source_path: 'stream-jobs/a/source.mp4',
    source_sha256: 'deadbeef',
    source_url: 'https://clips.twitch.tv/x',
    title: 't',
    probe: { width: 1920, height: 1080, duration_seconds: 30 },
    edit_plan: { schema_version: '1.0' },
    clip_count: 1,
    rendered_outputs: [{
      artifact_revision: '123e4567-e89b-42d3-a456-426614174000',
      variant: 'streamer-fullframe-nocam',
      clip_id: 'clip-1',
      artifact_name: 'clip-1.mp4',
      format: 'video/mp4',
      aspect_ratio: '9:16',
      video_url: '/api/streams/a/renders/streamer-fullframe-nocam/revisions/123e4567-e89b-42d3-a456-426614174000/videos/clip-1',
      render_status: 'rendered',
      stale: false,
      review_required: false,
    }],
    rendered_outputs_unavailable: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:01Z',
  };
  const { source_path: _path, source_sha256: _sha, ...want } = raw;
  assert.deepEqual(publicStreamJob(raw), want);
  assert.equal('source_path' in publicStreamJob(raw), false);
  assert.equal('source_sha256' in publicStreamJob(raw), false);
  assert.deepEqual(publicStreamJob(null), {});
});

test('editor asset projection drops media_key', () => {
  const raw = {
    id: 'a',
    sha256: 'abc',
    file_name: 'clip.mp4',
    origin: 'upload',
    origin_job_id: 'j',
    origin_variant: 'viral-60-clean',
    origin_name: 'clip.mp4',
    probe: { has_audio: true },
    media_key: 'editor-assets/a/media.mp4',
    created_at: '2026-01-01T00:00:00Z',
  };
  const { media_key: _key, ...want } = raw;
  assert.deepEqual(publicEditorAsset(raw), want);
});
