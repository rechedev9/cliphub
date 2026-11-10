import assert from 'node:assert/strict';
import { test } from 'node:test';
import { attackHref, lastMatchText, siteZones, warningText, windowTramo } from './attack.ts';

const MATCH = '6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c';
const refs = (n: number) => Array.from({ length: n }, (_, i) => ({ match_id: MATCH, round: i + 1, tick: 100 }));

test('a finding opens its rounds with the zones, the players and the tramo it names', () => {
  const href = attackHref(
    { team_name: 'Vitality', map: 'de_nuke' },
    { refs: refs(2), what: 'apEX en Rampa', zones: ['Ramp'], focus: ['76561198000000001'], tramo: windowTramo({ from_seconds: 75, to_seconds: 0 }) },
  );
  assert.ok(href !== undefined);
  const query = new URLSearchParams(href.slice('/replay?'.length));
  assert.deepEqual(query.getAll('r'), [`${MATCH}:1`, `${MATCH}:2`]);
  assert.equal(query.get('t'), 'Vitality · apEX en Rampa');
  assert.deepEqual(query.getAll('zona'), ['Ramp']);
  assert.equal(query.get('jugador'), '76561198000000001');
  assert.equal(query.get('tramo'), '75-115');
  assert.equal(attackHref({ team_name: 'Vitality', map: 'de_nuke' }, { refs: refs(0), what: 'nada' }), undefined);
});

test('site keys mark their bombsites and warnings read in Spanish', () => {
  assert.deepEqual(siteZones('both'), ['BombsiteA', 'BombsiteB']);
  assert.deepEqual(siteZones('none'), []);
  assert.match(warningText({ kind: 'stale', days: 70 }), /hace 70 días/);
  assert.match(warningText({ kind: 'roster_changed', joined: ['nuevo'], left: [] }), /entraron nuevo/);
  assert.match(warningText({ kind: 'capped', matches: 30, available: 41 }), /30 partidas más recientes de 41/);
  assert.equal(lastMatchText(1), 'Última partida hace 1 día');
  assert.equal(lastMatchText(0), 'Última partida hoy');
});
