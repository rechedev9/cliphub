import assert from 'node:assert/strict';
import { test } from 'node:test';
import { DEFAULT_HEAT, heatChoiceFromQuery, writeHeatChoice } from './heat-choice.ts';
import { teamFocusFromQuery, toggleTeamFocus, writeTeamFocus } from './player-focus.ts';

test('the heat choice round-trips through the keys the MCP studio_path writes', () => {
  const kill = writeHeatChoice('forma=calor&tramo=0-55', { kind: 'kill', killAt: 'victim', grenades: ['smoke'], floor: 'both' });
  assert.equal(kill, 'forma=calor&tramo=0-55&cuenta=kill&baja=victim');
  assert.deepEqual(heatChoiceFromQuery(new URLSearchParams(kill)), { kind: 'kill', killAt: 'victim', grenades: [], floor: 'both' });
  const utility = writeHeatChoice(kill, { kind: 'utility', killAt: 'victim', grenades: ['smoke', 'he'], floor: 'both' });
  assert.equal(utility, 'forma=calor&tramo=0-55&cuenta=utility&granada=smoke&granada=he');
  assert.equal(writeHeatChoice(utility, DEFAULT_HEAT), 'forma=calor&tramo=0-55');
});

test('an unknown kind, spot or grenade reads as the default', () => {
  assert.deepEqual(heatChoiceFromQuery(new URLSearchParams('cuenta=nope&baja=victim')), DEFAULT_HEAT);
  assert.deepEqual(heatChoiceFromQuery(new URLSearchParams('cuenta=kill&baja=elbow')).killAt, 'killer');
  assert.deepEqual(heatChoiceFromQuery(new URLSearchParams('cuenta=utility&granada=decoy&granada=flash&granada=flash')).grenades, ['flash']);
});

test('the floor is kept as piso and an unknown one reads as both', () => {
  const lower = writeHeatChoice('forma=calor', { ...DEFAULT_HEAT, floor: 'lower' });
  assert.equal(lower, 'forma=calor&piso=inferior');
  assert.equal(heatChoiceFromQuery(new URLSearchParams(lower)).floor, 'lower');
  assert.equal(heatChoiceFromQuery(new URLSearchParams('piso=superior')).floor, 'default');
  assert.equal(heatChoiceFromQuery(new URLSearchParams('piso=sotano')).floor, 'both');
  assert.equal(writeHeatChoice(lower, DEFAULT_HEAT), 'forma=calor');
});

test('the team focus keeps SteamID64s only, once each and at most ten', () => {
  const a = '76561198000000001';
  const b = '76561198000000002';
  const query = writeTeamFocus('tab=replay&jugador=1', [a, b]);
  assert.equal(query, `tab=replay&jugador=${a}&jugador=${b}`);
  assert.deepEqual(teamFocusFromQuery(new URLSearchParams(`${query}&jugador=${a}&jugador=slot0`)), [a, b]);
  assert.deepEqual(toggleTeamFocus([a, b], a), [b]);
  assert.deepEqual(toggleTeamFocus([b], a), [b, a]);
  const many = Array.from({ length: 12 }, (_, i) => `765611980000000${String(i).padStart(2, '0')}`);
  assert.equal(teamFocusFromQuery(new URLSearchParams(many.map((id) => `jugador=${id}`).join('&'))).length, 10);
});
