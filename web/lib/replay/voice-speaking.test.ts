import test from 'node:test';
import assert from 'node:assert/strict';
import { speakingSteamIds } from './voice-speaking.ts';

const tracks = [
  { track: 1, steamid64: '7656', speech: [{ start_ms: 1_000, end_ms: 2_000 }, { start_ms: 5_000, end_ms: 6_000 }, { start_ms: 9_000, end_ms: 9_500 }] },
  { track: 2, steamid64: '7657', speech: [{ start_ms: 0, end_ms: 10_000 }] },
  { track: 3, speech: [{ start_ms: 0, end_ms: 10_000 }] },
];

test('speakingSteamIds marks the tied, unmuted tracks inside a speech span', () => {
  assert.deepEqual([...speakingSteamIds({ tracks, muted: new Set(), audioMs: 5_500 })].sort(), ['7656', '7657']);
  assert.deepEqual([...speakingSteamIds({ tracks, muted: new Set([2]), audioMs: 5_500 })], ['7656']);
  assert.deepEqual([...speakingSteamIds({ tracks, muted: new Set([2]), audioMs: 3_000 })], []);
  // A short pause after a word keeps the mark.
  assert.deepEqual([...speakingSteamIds({ tracks, muted: new Set([2]), audioMs: 6_200 })], ['7656']);
  assert.deepEqual([...speakingSteamIds({ tracks, muted: new Set([2]), audioMs: -100 })], []);
});
