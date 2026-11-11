import assert from 'node:assert/strict';
import test from 'node:test';
import type { TeamGrenade } from '../api/grenades.ts';
import {
  lineupSaveError,
  lineupSpot,
  lineupThrow,
  parseSetpos,
  practiceBlockReason,
  sampleSetpos,
  setposCommand,
  techniqueLabel,
} from './lineups.ts';

test('parseSetpos reads what getpos prints', () => {
  assert.deepEqual(parseSetpos('setpos -1012.03 -2233.97 -167.97;setang 7.38 -68.11 0.00'), {
    origin: [-1012.03, -2233.97, -167.97],
    pitch: 7.38,
    yaw: -68.11,
  });
});

test('parseSetpos tolerates _exact, extra spaces, a missing roll and either order', () => {
  const want = { origin: [100, -200.5, 64], pitch: -12.5, yaw: 45 };
  assert.deepEqual(parseSetpos('  setpos_exact   100  -200.5 64 ;  setang_exact -12.5 45 0  '), want);
  assert.deepEqual(parseSetpos('setang -12.5 45\nsetpos 100 -200.5 64'), want);
  assert.deepEqual(parseSetpos('SETPOS 100 -200.5 64; SETANG -12.5 45'), want);
});

test('parseSetpos brings demo angles into setang ranges', () => {
  const parsed = parseSetpos('setpos 0 0 0;setang 350 270 0');
  assert.ok(parsed);
  assert.equal(parsed.pitch, -10);
  assert.equal(parsed.yaw, -90);
});

test('parseSetpos refuses text without both commands or off the map', () => {
  assert.equal(parseSetpos(''), null);
  assert.equal(parseSetpos('setpos 1 2 3'), null);
  assert.equal(parseSetpos('setang 1 2 0'), null);
  assert.equal(parseSetpos('setpos 1 2;setang 1 2 0'), null);
  assert.equal(parseSetpos('setpos 1 2 99999999;setang 1 2 0'), null);
  assert.equal(parseSetpos('getpos'), null);
});

test('setposCommand writes two decimals and a zero roll, and round trips through parseSetpos', () => {
  const aim = { origin: [-1012.031, 2233.9, -0.001] satisfies [number, number, number], pitch: 7.384, yaw: -68.1 };
  const command = setposCommand(aim);
  assert.equal(command, 'setpos -1012.03 2233.90 0.00;setang 7.38 -68.10 0');
  assert.deepEqual(parseSetpos(command), { origin: [-1012.03, 2233.9, 0], pitch: 7.38, yaw: -68.1 });
});

test('lineupSpot hands a CT molotov as an incendiary and keeps setang order', () => {
  const base = { title: 'Molo a Banana', origin: [1, 2, 3] satisfies [number, number, number], pitch: -10, yaw: 90 };
  assert.deepEqual(lineupSpot({ ...base, grenade: 'molotov', side: 'CT' }), {
    label: 'Molo a Banana',
    pos: [1, 2, 3],
    ang: [-10, 90],
    grenade: 'incendiary',
  });
  assert.equal(lineupSpot({ ...base, grenade: 'molotov', side: 'T' }).grenade, 'molotov');
  assert.equal(lineupSpot({ ...base, title: 'x'.repeat(300), grenade: 'smoke', side: 'T' }).label.length, 200);
});

test('practiceBlockReason asks for one map and at most 20 lineups', () => {
  assert.equal(practiceBlockReason([{ map: 'de_mirage' }, { map: 'de_mirage' }]), null);
  assert.match(practiceBlockReason([{ map: 'de_mirage' }, { map: 'de_inferno' }]) ?? '', /mismo mapa/);
  assert.match(practiceBlockReason(Array.from({ length: 21 }, () => ({ map: 'de_nuke' }))) ?? '', /20/);
  assert.notEqual(practiceBlockReason([]), null);
});

test('techniqueLabel reads like a player says it', () => {
  assert.equal(techniqueLabel({ movement: 'running', jump: true, click: 'left' }), 'Corriendo, salto, clic izquierdo');
  assert.equal(techniqueLabel({ movement: 'standing', jump: false, click: 'both' }), 'Parado, los dos clics');
});

test('lineupSaveError explains the server code', () => {
  assert.match(lineupSaveError('throw_missing'), /analizar la demo/);
  assert.equal(lineupSaveError(undefined), 'No se pudo guardar el lineup.');
});

test('lineupThrow needs the origin, the view and the throw tick', () => {
  const grenade = {
    match_id: 'm', round: 4, tick: 900, seconds: 10, kind: 'smoke', side: 'T', thrower_steamid64: '1', thrower_name: 'a',
    origin: [1, 2, 3], throw_tick: 840, view: [90, -10], landing: [4, 5, 6], buy: 'full', round_won: true,
  } as TeamGrenade;
  assert.deepEqual(lineupThrow(grenade), { match_id: 'm', round: 4, throw_tick: 840 });
  assert.equal(lineupThrow({ ...grenade, throw_tick: null }), null);
  assert.equal(lineupThrow({ ...grenade, view: null }), null);
  assert.equal(lineupThrow({ ...grenade, origin: null }), null);
});

// The same vector is pinned in cmd/zv-mcp (TestSetposCommandMatchesTheWebByteForByte): both sides must agree.
test('setposCommand has a dot decimal, no thousands separator, no -0 and rounds a half away from zero', () => {
  assert.equal(setposCommand({ origin: [-0.001, 0.125, -167.96875], pitch: -0.625, yaw: 1.005 }), 'setpos 0.00 0.13 -167.97;setang -0.63 1.00 0');
  assert.equal(setposCommand({ origin: [-1234567.891, 1000, -0.004], pitch: 0, yaw: -0 }), 'setpos -1234567.89 1000.00 0.00;setang 0.00 0.00 0');
});

test('sampleSetpos normalizes the demo angles and refuses a number that is not finite', () => {
  assert.equal(sampleSetpos({ x: 1, y: 2, z: 3, yaw: 359.5, pitch: 270 }), 'setpos 1.00 2.00 3.00;setang -90.00 -0.50 0');
  assert.equal(sampleSetpos({ x: 1, y: 2, z: 3, yaw: -180 }), 'setpos 1.00 2.00 3.00;setang 0.00 180.00 0');
  assert.equal(sampleSetpos({ x: Number.NaN, y: 2, z: 3, yaw: 0 }), null);
  assert.equal(sampleSetpos({ x: 1, y: 2, z: 3, yaw: Number.POSITIVE_INFINITY, pitch: 0 }), null);
});
