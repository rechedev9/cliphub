import test from 'node:test';
import assert from 'node:assert/strict';
import type { PlayerSidePositions } from '../api/team-insights.ts';
import { awpText, roleSentence } from './positions.ts';

const rate = (count: number, total: number) => ({ count, total, pct: total > 0 ? (100 * count) / total : 0, reliable: total >= 10 });

function side(overrides: Partial<PlayerSidePositions>): PlayerSidePositions {
  return {
    side: 'CT', rounds: 50, sampled: 45,
    zones: [{ zone: 'A', rounds: rate(9, 45) }, { zone: 'B', rounds: rate(29, 45) }, { zone: 'other', rounds: rate(7, 45) }],
    places: [], awp_kills: rate(0, 30), awp_rounds: rate(0, 50),
    ...overrides,
  };
}

test('positions: an anchor reads as its site share of the sampled rounds', () => {
  assert.equal(roleSentence(side({ role: 'b_anchor' }), 'Mirage'), 'Ancla B en Mirage, 64 % de 45 rondas');
});

test('positions: a rotator reads as its split, a T side as the zone it leans to', () => {
  assert.equal(roleSentence(side({ role: 'rotator' }), 'Mirage'), 'Rotador en Mirage: 20 % en A y 64 % en B de 45 rondas');
  assert.equal(roleSentence(side({ side: 'T' }), 'Nuke'), 'Juega en B en Nuke, 64 % de 45 rondas');
  assert.equal(roleSentence(side({ sampled: 0 }), 'Nuke'), 'Sin posiciones leídas en Nuke');
});

test('positions: AWP share only when the player killed with one', () => {
  assert.equal(awpText(side({})), null);
  assert.equal(
    awpText(side({ awp_rounds: rate(12, 40), awp_kills: rate(18, 52) })),
    'Mata con AWP en 12 de 40 rondas (30 %), 18 de sus 52 bajas',
  );
});
