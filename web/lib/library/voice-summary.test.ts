import test from 'node:test';
import assert from 'node:assert/strict';
import type { VoiceSync } from '../api/voice.ts';
import { resyncOutcome, voiceIndex, voiceSyncTitle } from './voice-summary.ts';

function sync(patch: Partial<VoiceSync>): VoiceSync {
  return { state: 'auto', method: 'kills', offset_ms: 0, auto_offset_ms: 0, confidence: 0.874, ...patch };
}

test('the voice tooltip names how the recording is synced', () => {
  assert.equal(voiceSyncTitle(sync({ state: 'manual' })), 'Voz sincronizada a mano');
  assert.equal(voiceSyncTitle(sync({})), 'Voz sincronizada sola (confianza 87 %)');
  assert.equal(voiceSyncTitle(sync({ state: 'unsynced' })), 'Voz sin sincronizar');
});

test('a resync that still fails keeps the reason, one that works says how it synced', () => {
  assert.deepEqual(resyncOutcome(sync({ state: 'unsynced', note: 'Es de otra partida.' })), {
    tone: 'warning',
    text: 'Sigue sin sincronizar. Es de otra partida.',
  });
  assert.deepEqual(resyncOutcome(sync({ state: 'unsynced' })), { tone: 'warning', text: 'Sigue sin sincronizar.' });
  assert.equal(resyncOutcome(sync({})).tone, 'ok');
});

test('the index finds a summary by match', () => {
  const summary = { match_id: 'm1', tracks: 2, sync: sync({}) };
  assert.equal(voiceIndex([summary]).get('m1'), summary);
  assert.equal(voiceIndex([summary]).get('m2'), undefined);
});
