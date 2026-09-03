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
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:01Z',
  };
  const { source_path: _path, source_sha256: _sha, ...want } = raw;
  assert.deepEqual(publicStreamJob(raw), want);
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
