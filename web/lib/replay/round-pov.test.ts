import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { isFullDemoOptions } from '../full-demo-plan.ts';
import { RoundPovFailure, roundPovBlocked, roundPovDone, roundPovError, roundPovFollowing, roundPovKey, roundPovLabel, roundPovOptions } from './round-pov.ts';
import type { RoundPovRun } from './round-pov.ts';

const defaults: unknown = JSON.parse(readFileSync(new URL('../full-demo-go-defaults.fixture.json', import.meta.url), 'utf8'));
const run: RoundPovRun = { key: 'm:7:76561198000000001', round: 7, name: 'ropz', phase: 'preparing' };

test('a round POV is that one round, plain', () => {
  assert.ok(isFullDemoOptions(defaults));
  const options = roundPovOptions(defaults, 7);
  assert.deepEqual(options.rounds, [7]);
  assert.equal(options.transitions?.enabled, false);
  assert.equal(options.bumpers, undefined);
  assert.ok(isFullDemoOptions(options));
});

test('the key tells one player, round and match apart', () => {
  const key = roundPovKey({ matchId: 'm', round: 7, steamId: '1' });
  assert.notEqual(key, roundPovKey({ matchId: 'm', round: 8, steamId: '1' }));
  assert.notEqual(key, roundPovKey({ matchId: 'm', round: 7, steamId: '2' }));
});

test('a run follows its video and shows the percent only while it works', () => {
  const recording = roundPovFollowing(run, { status: 'recording', captureProgress: { done: 3, total: 10, percent: 30.4 } });
  assert.equal(roundPovLabel(recording), 'Grabando 30%');
  assert.equal(roundPovDone(recording), false);
  const ready = roundPovFollowing(recording, { status: 'ready' });
  assert.equal(roundPovLabel(ready), 'Listo');
  assert.equal(ready.percent, undefined);
  assert.equal(roundPovDone(ready), true);
});

test('a failed video says why', () => {
  const failed = roundPovFollowing(run, { status: 'failed', failureReason: 'CS2 se cerró' });
  assert.ok(failed.error !== undefined && failed.error.length > 0);
  assert.equal(roundPovDone(failed), true);
});

test('failures read in Spanish, by code first', () => {
  const busy = Object.assign(new Error('Wait for the active job'), { code: 'generate_work_active' });
  assert.equal(roundPovError(busy), 'Este jugador ya tiene una grabación en marcha; espera a que termine.');
  assert.equal(roundPovError(roundPovBlocked('pov_dead_in_freeze')), 'Esta ronda no se puede grabar desde su POV.');
  assert.equal(roundPovError(new RoundPovFailure('Ya hay un Full Demo en curso.')), 'Ya hay un Full Demo en curso.');
  assert.equal(roundPovError(new Error('boom')), 'No se pudo preparar la grabación. Inténtalo otra vez.');
});
