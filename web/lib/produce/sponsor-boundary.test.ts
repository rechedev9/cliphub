import test from 'node:test';
import assert from 'node:assert/strict';
import { certifiedRoundId, isPrepareAbort } from './sponsor-boundary.ts';

const rounds = ['round-001', 'round-002'];

test('keeps a certified after_round_id that is still a candidate', () => {
  assert.equal(certifiedRoundId(rounds, 'round-002'), 'round-002');
});

test('falls back to the first candidate when the current round is gone or empty', () => {
  assert.equal(certifiedRoundId(rounds, 'round-009'), 'round-001');
  assert.equal(certifiedRoundId(rounds, ''), 'round-001');
  assert.equal(certifiedRoundId([], 'round-002'), undefined);
});

test('isPrepareAbort is true only for AbortError', () => {
  assert.equal(isPrepareAbort(new DOMException('La preparación se canceló.', 'AbortError')), true);
  assert.equal(isPrepareAbort(new Error('No se pudieron preparar las rondas.')), false);
});
