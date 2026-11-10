import assert from 'node:assert/strict';
import { test } from 'node:test';
import { FILTER_GROUPS, groupFields, unreachableFields, visibleGroups } from './scope-filter-groups.ts';

test('every scope field has a control in the row or in a menu group', () => {
  assert.deepEqual(unreachableFields(), []);
});

test('a group shows only when the view reads one of its fields', () => {
  assert.deepEqual(visibleGroups(['map', 'side']).map((group) => group.id), []);
  assert.deepEqual(visibleGroups(['map', 'weapon', 'postPlant']).map((group) => group.id), ['weapon', 'postplant']);
});

test('the buy matchup group needs both buys', () => {
  assert.equal(visibleGroups(['buy']).some((group) => group.id === 'matchup'), false);
  assert.equal(visibleGroups(['buy', 'opponentBuy']).some((group) => group.id === 'matchup'), true);
});

test('a group lists only the fields the view reads', () => {
  const weapon = FILTER_GROUPS.find((group) => group.id === 'weapon');
  assert.ok(weapon);
  assert.deepEqual(groupFields(weapon, ['awp', 'tactic']), ['awp']);
});
