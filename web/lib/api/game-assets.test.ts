import test from 'node:test';
import assert from 'node:assert/strict';
import {
  EQUIPMENT_ICON_NAMES,
  equipmentIconForWeapon,
  equipmentIconUrl,
  isEquipmentIconName,
  isRadarLayer,
  radarImageUrl,
} from './game-assets.ts';

test('radarImageUrl and equipmentIconUrl point at the Studio routes', () => {
  assert.equal(radarImageUrl('de_nuke'), '/api/maps/de_nuke/radar?layer=upper');
  assert.equal(radarImageUrl('de_nuke', 'lower'), '/api/maps/de_nuke/radar?layer=lower');
  assert.equal(equipmentIconUrl('m4a1_silencer'), '/api/equipment/m4a1_silencer/icon');
  assert.ok(isRadarLayer('lower'));
  assert.ok(!isRadarLayer('middle'));
});

test('every known icon name passes the orchestrator name rule', () => {
  for (const name of EQUIPMENT_ICON_NAMES) assert.ok(isEquipmentIconName(name), name);
  for (const bad of ['AK47', '../x', 'knife__x', '_ak', 'a'.repeat(41)]) assert.ok(!isEquipmentIconName(bad), bad);
});

test('equipmentIconForWeapon maps tactical event weapons to icons', () => {
  const cases: [string | undefined, string | null][] = [
    ['ak47', 'ak47'],
    ['weapon_m4a1_silencer', 'm4a1_silencer'],
    ['m4a1', 'm4a1'],
    ['hkp2000', 'hkp2000'],
    ['incgrenade', 'incgrenade'],
    ['inferno', 'inferno'],
    ['Kevlar Vest', 'kevlar'],
    ['Kevlar + Helmet', 'assaultsuit'],
    ['Defuse Kit', 'defuser'],
    ['knife_karambit', 'knife_karambit'],
    ['knife_some_future_skin', 'knife'],
    ['taser', 'taser'],
    ['c4', 'c4'],
    ['world', 'world'],
    ['unknown', null],
    [undefined, null],
  ];
  for (const [weapon, want] of cases) assert.equal(equipmentIconForWeapon(weapon), want, String(weapon));
});
