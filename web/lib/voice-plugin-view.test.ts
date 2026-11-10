import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryMatch } from './api/library.ts';
import type { VoiceRecording } from './api/voice-plugin.ts';
import { recordingMatchChoices, recordingTitle } from './voice-plugin-view.ts';

function match(id: string, map: string, playedAt: string): LibraryMatch {
  return { id, demo_id: id, map, played_at: playedAt, source: 'scrim', tickrate: 64, duration_ticks: 0, round_count: 24, teams: [], created_at: playedAt, updated_at: playedAt };
}

const recording: VoiceRecording = {
  id: '20261109-200000-ab12', status: 'pending', started_at: '2026-11-09T20:00:00Z', ended_at: '2026-11-09T20:41:05Z',
  duration_ms: 2_465_000, map: 'de_mirage', trigger: 'auto', score_ct: 13, score_t: 9,
  tracks: [{ file: '01-a.pcm', nickname: 'a', duration_ms: 1 }, { file: '02-b.pcm', nickname: 'b', duration_ms: 1 }],
};

test('recordingMatchChoices puts the same map first, then the closest in time', () => {
  const matches = [
    match('far-mirage', 'de_mirage', '2026-11-01T20:00:00Z'),
    match('near-nuke', 'de_nuke', '2026-11-09T20:05:00Z'),
    match('near-mirage', 'de_mirage', '2026-11-09T20:10:00Z'),
  ];
  assert.deepEqual(recordingMatchChoices(recording, matches).map((m) => m.id), ['near-mirage', 'far-mirage', 'near-nuke']);
  assert.deepEqual(recordingMatchChoices({ ...recording, map: undefined }, matches, 2).map((m) => m.id), ['near-nuke', 'near-mirage']);
});

test('recordingTitle names the map, tracks and length', () => {
  assert.match(recordingTitle(recording), / · de_mirage · 2 pistas · 41:05$/);
  assert.match(recordingTitle({ ...recording, map: undefined, tracks: recording.tracks.slice(0, 1), duration_ms: 3_725_000 }), / · 1 pista · 1:02:05$/);
});
