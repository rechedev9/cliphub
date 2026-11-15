import assert from 'node:assert/strict';
import test from 'node:test';
import type { Play } from '../api/types.ts';
import { playDescription } from './play-details.ts';

const base: Play = { id: 's', matchId: 'm', label: 's', kind: 'highlight', round: 4, kills: 3, weapon: 'AWP' };

test('the description names the clutch result, the kill types and the win chance swing', () => {
  assert.equal(
    playDescription({ ...base, clutchVs: 3, clutchWon: true, reasons: ['clutch', 'fast_triple', 'noscope'], winProbSwing: 0.456 }),
    '3 kills · AWP · Clutch 1v3 ganado · Triple en 10 s · Noscope · +46 % prob. de ronda',
  );
  assert.equal(playDescription({ ...base, clutchVs: 2, clutchWon: false }), '3 kills · AWP · Clutch 1v2 perdido');
  assert.equal(playDescription({ ...base, clutchVs: 1 }), '3 kills · AWP · Clutch 1v1', 'an older plan has no result');
});
