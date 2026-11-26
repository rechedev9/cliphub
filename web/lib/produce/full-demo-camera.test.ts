import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { isFullDemoSnapshot } from '../full-demo-plan.ts';
import { cameraEmptyReason, cameraIdRound, cameraOmittedText, fullDemoCameraListed, fullDemoCameraMoments } from './full-demo-camera.ts';
import { fullDemoBriefItems } from './full-demo-brief.ts';
import { fullDemoWarningText } from './full-demo-notices.ts';

function planDocument() {
  const value: unknown = JSON.parse(readFileSync(new URL('../full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(value));
  return value.document;
}

test('moments show the round, the enemy and the round clock of the kill', () => {
  const base = planDocument();
  const round = base.rounds[0];
  assert.ok(round);
  const kill = round.live_start_tick + Math.round(base.clock.tick_rate * 48);
  const window = { id: `${round.round_id}-camera-${kill}`, round_id: round.round_id, round: round.source_round_number, kill_tick: kill, start_tick: kill - 320, end_tick: kill + 64, enemy_steamid64: '76561198000000001', enemy_name: 'Rival', score: 1, move_units: 0 };
  const moments = fullDemoCameraMoments({ ...base, second_camera: { windows: [window, { ...window, id: 'x', enemy_name: '' }] } });
  assert.deepEqual(moments.map((moment) => [moment.round, moment.enemy, moment.time]), [[round.source_round_number, 'Rival', '0:48'], [round.source_round_number, 'Rival', '0:48']]);
  assert.deepEqual(fullDemoCameraMoments(base), []);
});

test('notices and omissions speak Spanish without dashes', () => {
  assert.match(fullDemoWarningText({ code: 'second_camera_skipped', message: '' }), /sale sin ella/);
  assert.match(fullDemoWarningText({ code: 'second_camera_unavailable', message: '' }), /movimientos de los enemigos/);
  assert.match(fullDemoWarningText({ code: 'second_camera_omitted', message: '', round_id: 'round-012' }), /sale sin ella\. Ronda 12\./);
  assert.equal(cameraIdRound('round-012-camera-81234'), 12);
  assert.equal(cameraIdRound('nope'), null);
  assert.match(cameraOmittedText('round-012-camera-81234'), /de la ronda 12 no se pudo grabar/);
  assert.doesNotMatch(cameraOmittedText('round-012-camera-81234'), /[–—]/);
  assert.match(cameraEmptyReason({ ...planDocument(), warnings: [{ code: 'second_camera_skipped', message: '' }] }), /Ninguna baja/);
  assert.match(cameraEmptyReason({ ...planDocument(), warnings: [] }), /quitado/);
});

test('the brief row for the second camera follows the option and the skip', () => {
  const options = planDocument().options;
  const row = (on: boolean, skipped: boolean) => fullDemoBriefItems({ options: on ? { ...options, second_camera: { enabled: true, max_windows: 5 } } : options, voiceAvailability: null, cameraSkipped: skipped }).find((item) => item.label === 'Segunda cámara')?.value;
  assert.equal(row(false, false), 'No');
  assert.equal(row(true, false), 'Sí');
  assert.equal(row(true, true), 'Omitido');
});

test('the moment list hides when the cap or the rounds no longer match the plan', () => {
  const base = planDocument();
  const camera = { enabled: true, max_windows: 5 };
  const planned = { ...base, options: { ...base.options, rounds: [2, 4], second_camera: camera } };
  assert.equal(fullDemoCameraListed(planned, { ...base.options, rounds: [4, 2], second_camera: camera }), planned);
  assert.equal(fullDemoCameraListed(planned, { ...base.options, rounds: [2, 4], second_camera: { ...camera, max_windows: 3 } }), null);
  assert.equal(fullDemoCameraListed(planned, { ...base.options, rounds: [2, 4, 7], second_camera: camera }), null);
  assert.equal(fullDemoCameraListed(base, { ...base.options, second_camera: camera }), null);
  assert.equal(fullDemoCameraListed(null, { ...base.options, second_camera: camera }), null);
});
