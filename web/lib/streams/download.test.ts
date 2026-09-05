import test from 'node:test';
import assert from 'node:assert/strict';
import { streamVideoFilename } from './download.ts';
import { streamsApi } from '../api/streams.ts';
test('downloads keep readable Unicode titles and remove path characters and reserved names', () => {
  assert.equal(streamVideoFilename('DALE NIÑO !'), 'DALE NIÑO !.mp4');
  assert.equal(streamVideoFilename('clip/uno:final?'), 'clip-uno-final-.mp4');
  assert.equal(streamVideoFilename('CON'), 'Short-CON.mp4');
  assert.equal(streamVideoFilename('  ... '), 'Short.mp4');
  assert.equal(streamVideoFilename('x'.repeat(300)).length, 104);
});
test('a new export is distinct from a previously saved video with the same clip ID', () => {
  const a = streamsApi.videoUrl('job', 'streamer-fullframe-nocam', 'clip', '2026-09-05T10:00:00Z');
  const b = streamsApi.videoUrl('job', 'streamer-fullframe-nocam', 'clip', '2026-09-05T11:00:00Z');
  assert.notEqual(a, b);
  assert.equal(new URL(a, 'https://studio.local').searchParams.get('v'), '2026-09-05T10:00:00Z');
});
