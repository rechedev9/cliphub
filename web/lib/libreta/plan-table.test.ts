import assert from 'node:assert/strict';
import test from 'node:test';
import type { LibraryPlay } from '../api/stratbook.ts';
import { PLAN_ALL_TEAMS, PLAN_NO_TEAM, busiestMap, planColumns } from './plan-table.ts';

function play(over: Partial<LibraryPlay>): LibraryPlay {
  return {
    id: over.title ?? 'x',
    title: 'x',
    team_id: null,
    map: 'de_mirage',
    side: 'T',
    kind: 'execute',
    site: 'A',
    origin: 'manual',
    positions_seconds: 0,
    grenades: [],
    positions: [],
    notes: '',
    examples: [],
    status: 'active',
    tags: [],
    roles: [],
    created_at: '',
    updated_at: '',
    ...over,
  };
}

const plays = [
  play({ title: 'B split', kind: 'split', team_id: 'vit' }),
  play({ title: 'A exec', team_id: 'vit' }),
  play({ title: 'Vieja', status: 'archived' }),
  play({ title: 'Sin forma', kind: 'unknown' }),
  play({ title: 'Nuke', map: 'de_nuke' }),
  play({ title: 'CT hold', side: 'CT', kind: 'hold' }),
];

test('planColumns puts each play of the scope in its kind column, archived ones out', () => {
  const columns = planColumns(plays, { team: PLAN_ALL_TEAMS, map: 'de_mirage', side: 'T' });
  assert.deepEqual(
    columns.map((column) => [column.kind, column.plays.map((p) => p.title)]),
    [['execute', ['A exec']], ['default', []], ['split', ['B split']], ['fast', []], ['eco_rush', []], ['save', []], ['unknown', ['Sin forma']]],
  );
});

test('planColumns filters by team and drops an empty "Sin forma" column', () => {
  const vit = planColumns(plays, { team: 'vit', map: 'de_mirage', side: 'T' });
  assert.equal(vit.flatMap((column) => column.plays).length, 2);
  assert.equal(vit.at(-1)?.kind, 'save');
  const none = planColumns(plays, { team: PLAN_NO_TEAM, map: 'de_mirage', side: 'T' });
  assert.deepEqual(none.flatMap((column) => column.plays).map((p) => p.title), ['Sin forma']);
});

test('busiestMap picks the map with most live plays', () => {
  assert.equal(busiestMap(plays, 'de_dust2'), 'de_mirage');
  assert.equal(busiestMap([], 'de_dust2'), 'de_dust2');
});
