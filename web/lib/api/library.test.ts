import test from 'node:test';
import assert from 'node:assert/strict';
import { playerPositionsPath, spatialPath } from './library.ts';

const ID = '11111111-1111-4111-8111-111111111111';

test('a match view carries the tactical filter, then its own query', () => {
  const path = spatialPath({ kind: 'match', matchId: ID, filter: 'team=ct-start&buy=full' }, 'heatmap', new URLSearchParams('kind=death'));
  assert.equal(path, `/matches/${ID}/tactical/heatmap?team=ct-start&buy=full&kind=death`);
  assert.equal(spatialPath({ kind: 'match', matchId: ID, filter: '' }, 'overlay', new URLSearchParams()), `/matches/${ID}/tactical/overlay`);
});

test('a team view carries the map, the side, the matches, the lineup and the buys', () => {
  const path = spatialPath(
    { kind: 'team', teamId: ID, query: { map: 'de_nuke', side: 'T', match_ids: ['a', 'b'], lineup: ['1', '2'], buys: ['full'], opponent_ids: [] } },
    'overlay',
    new URLSearchParams('limit=20'),
  );
  assert.equal(path, `/teams/${ID}/overlay?map=de_nuke&side=T&match_id=a&match_id=b&lineup=1&lineup=2&buy=full&limit=20`);
});

test('a player positions path carries only the map and the dates', () => {
  assert.equal(playerPositionsPath('7656', {}), '/players/7656/positions');
  assert.equal(playerPositionsPath('7656', { map: 'de_nuke', to: 'x' }), '/players/7656/positions?map=de_nuke&to=x');
});
