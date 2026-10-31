import test from 'node:test';
import assert from 'node:assert/strict';
import { placeLabel } from './tactical-labels.ts';

test('CS2 place tokens read as the callouts players say', () => {
  assert.equal(placeLabel('BombsiteA'), 'Bombsite A');
  assert.equal(placeLabel('CTSpawn'), 'CT Spawn');
  assert.equal(placeLabel('TSpawn'), 'T Spawn');
  assert.equal(placeLabel('BDoors'), 'B Doors');
  assert.equal(placeLabel('ARamp'), 'A Ramp');
  assert.equal(placeLabel('CTSideUpper'), 'CT Side Upper');
  assert.equal(placeLabel('PalaceInterior'), 'Palace Interior');
  assert.equal(placeLabel('TopofMid'), 'Top of Mid');
  assert.equal(placeLabel('BackofB'), 'Back of B');
  assert.equal(placeLabel('SnipersNest'), "Sniper's Nest");
  assert.equal(placeLabel('Banana'), 'Banana');
  assert.equal(placeLabel(''), '');
});
