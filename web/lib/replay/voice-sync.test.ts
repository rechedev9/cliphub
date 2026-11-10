import test from 'node:test';
import assert from 'node:assert/strict';
import { audioSecondsAt, clampOffset, deltaLabel, formatVoiceMs, needsSeek, playbackRate, silentOutsideFreeze, tickOfAudioMs, trackVolume } from './voice-sync.ts';

test('the recording plays at demo time minus the offset, and is silent outside it', () => {
  const at = { tickrate: 64, offsetMs: 10_000, durationMs: 60_000 };
  assert.equal(audioSecondsAt({ ...at, tick: 64 * 15 }), 5);
  assert.equal(audioSecondsAt({ ...at, tick: 64 * 5 }), null);
  assert.equal(audioSecondsAt({ ...at, tick: 64 * 71 }), null);
  assert.equal(audioSecondsAt({ ...at, offsetMs: -2000, tick: 0 }), 2);
});

test('a phrase lands on the demo tick its audio time plus the offset gives', () => {
  assert.equal(tickOfAudioMs(5000, { offsetMs: 10_000, tickrate: 64 }), 960);
  assert.equal(tickOfAudioMs(5000, { offsetMs: -5000, tickrate: 128 }), 0);
});

test('the audio moves only past the drift a playing or paused element allows', () => {
  assert.equal(needsSeek({ current: 10, target: 10.2, playing: true }), false);
  assert.equal(needsSeek({ current: 10, target: 10.4, playing: true }), true);
  assert.equal(needsSeek({ current: 10, target: 10.2, playing: false }), true);
});

test('the freeze-time filter silences the voice once the round goes live', () => {
  assert.equal(silentOutsideFreeze({ tick: 900, freezeOnly: true, freezeEndTick: 1000 }), false);
  assert.equal(silentOutsideFreeze({ tick: 1000, freezeOnly: true, freezeEndTick: 1000 }), true);
  assert.equal(silentOutsideFreeze({ tick: 5000, freezeOnly: false, freezeEndTick: 1000 }), false);
  assert.equal(silentOutsideFreeze({ tick: 5000, freezeOnly: true, freezeEndTick: null }), false);
});

test('the playback rate follows the replay speed within what audio can play', () => {
  assert.equal(playbackRate(2), 2);
  assert.equal(playbackRate(8), 4);
  assert.equal(playbackRate(0), 1);
});

test('offsets and deltas read in the adjust bar', () => {
  assert.equal(formatVoiceMs(62_500), '1:02,5');
  assert.equal(formatVoiceMs(-1200), '-0:01,2');
  assert.equal(formatVoiceMs(1200, true), '+0:01,2');
  assert.equal(deltaLabel(11_200, 10_000), 'Δ +1,2 s');
  assert.equal(deltaLabel(9_500, 10_000), 'Δ -0,5 s');
  assert.equal(deltaLabel(10_020, 10_000), 'Δ 0 s');
  assert.equal(clampOffset(9e9, 1000), 1000);
});

test('a muted track or a muted voice is silent', () => {
  assert.equal(trackVolume({ master: 0.6, muted: false, trackMuted: false }), 0.6);
  assert.equal(trackVolume({ master: 0.6, muted: true, trackMuted: false }), 0);
  assert.equal(trackVolume({ master: 2, muted: false, trackMuted: false }), 1);
});
