import test from 'node:test';
import assert from 'node:assert/strict';
import { equipmentId, equipmentItem, rowInventory } from './equipment.ts';
import { roundOutcome } from './round-outcome.ts';

test('equipment: demo, entity and display names map to one id', () => {
  assert.equal(equipmentId('weapon_m4a1_silencer'), 'm4a1silencer');
  assert.equal(equipmentId('M4A1-S'), 'm4a1s');
  assert.equal(equipmentItem('weapon_m4a1_silencer').label, 'M4A1-S');
  assert.equal(equipmentItem('m4a1').label, 'M4A4');
  assert.equal(equipmentItem('AWP').category, 'sniper');
  assert.equal(equipmentItem('weapon_knife_karambit').category, 'knife');
  assert.equal(equipmentItem('weapon_newgun').category, 'other');
});

test('equipment: a row lists the gun, the pistol, then each grenade by count', () => {
  const items = rowInventory([
    { item: 'flashbang', count: 2 },
    { item: 'knife', count: 1 },
    { item: 'glock', count: 1 },
    { item: 'smokegrenade', count: 1 },
    { item: 'ak47', count: 1 },
    { item: 'c4', count: 1 },
  ]);
  assert.deepEqual(items.map((item) => item.label), ['AK-47', 'Glock', 'Flash', 'Flash', 'Humo', 'C4']);
});

test('round outcome: end reasons map to the strip glyphs', () => {
  assert.equal(roundOutcome('bomb_exploded'), 'exploded');
  assert.equal(roundOutcome('bomb_defused'), 'defused');
  assert.equal(roundOutcome('t_eliminated_ct'), 'elimination');
  assert.equal(roundOutcome('time_expired'), 'time');
  assert.equal(roundOutcome('unknown'), 'other');
});
