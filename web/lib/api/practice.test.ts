import test from 'node:test';
import assert from 'node:assert/strict';
import { parsePracticeLaunchInput, parsePracticeLaunchResult, projectPracticeLaunch } from './practice.ts';

const SPOT = { label: 'Ventana', pos: [-1185.5, -1253, -167.97], ang: [-12.5, 45.25], grenade: 'smoke' };

test('parsePracticeLaunchInput keeps a valid request and drops unknown spot fields', () => {
  const parsed = parsePracticeLaunchInput({ map: 'de_mirage', spots: [{ ...SPOT, extra: 'x' }] });
  assert.deepEqual(parsed, {
    ok: true,
    value: { map: 'de_mirage', spots: [{ label: 'Ventana', pos: [-1185.5, -1253, -167.97], ang: [-12.5, 45.25], grenade: 'smoke' }] },
  });
});

test('parsePracticeLaunchInput rejects bad maps, counts, numbers and grenades', () => {
  const bad: unknown[] = [
    { map: 'de_dust2; quit', spots: [SPOT] },
    { map: 'cs_office', spots: [SPOT] },
    { map: 'de_nuke', spots: [] },
    { map: 'de_nuke', spots: Array.from({ length: 21 }, () => SPOT) },
    { map: 'de_nuke', spots: [{ ...SPOT, pos: [1, 2] }] },
    { map: 'de_nuke', spots: [{ ...SPOT, pos: [1, '2', 3] }] },
    { map: 'de_nuke', spots: [{ ...SPOT, ang: [Number.NaN, 0] }] },
    { map: 'de_nuke', spots: [{ ...SPOT, grenade: 'awp' }] },
    { map: 'de_nuke', spots: [{ ...SPOT, label: 7 }] },
    null,
  ];
  for (const body of bad) {
    assert.equal(parsePracticeLaunchInput(body).ok, false, JSON.stringify(body));
  }
});

test('projectPracticeLaunch camelCases the orchestrator body and the client reads it back', () => {
  const upstream = {
    launched: false,
    already_running: true,
    cfg_path: 'C:\\cs2\\game\\csgo\\cfg\\cliphub_practice.cfg',
    exec_command: 'exec cliphub_practice',
    next_key: 'KP_PLUS',
    spots: [
      { ...SPOT, alias: 'cliphub_spot_1', key: 'KP_END' },
      { label: 'Medio', pos: [0, 0, 0], ang: [0, 180], alias: 'cliphub_spot_10' },
    ],
    secret: 'drop me',
  };
  const projected = projectPracticeLaunch(upstream);
  assert.deepEqual(projected, {
    launched: false,
    alreadyRunning: true,
    cfgPath: 'C:\\cs2\\game\\csgo\\cfg\\cliphub_practice.cfg',
    execCommand: 'exec cliphub_practice',
    nextKey: 'KP_PLUS',
    spots: [
      { label: 'Ventana', pos: [-1185.5, -1253, -167.97], ang: [-12.5, 45.25], grenade: 'smoke', alias: 'cliphub_spot_1', key: 'KP_END' },
      { label: 'Medio', pos: [0, 0, 0], ang: [0, 180], alias: 'cliphub_spot_10' },
    ],
  });
  assert.deepEqual(parsePracticeLaunchResult(JSON.parse(JSON.stringify(projected))), projected);
  assert.equal(projectPracticeLaunch({ ...upstream, cfg_path: 1 }), null);
  assert.equal(projectPracticeLaunch({ ...upstream, spots: [{ label: 'x' }] }), null);
});
