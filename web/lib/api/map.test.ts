import assert from 'node:assert/strict';
import test from 'node:test';
import { planToPlays, prettifyWeapon } from './map.ts';

test('planToPlays removes duplicate timeline segments but preserves distinct plays', () => {
  const plays = planToPlays('job-1', {
    segments: [
      { id: 'first', round: 4, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] },
      { id: 'duplicate', round: 4, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] },
      { id: 'second', round: 4, tick_start: 210, tick_end: 260, kills: [{ weapon: 'awp' }] },
    ],
  });
  assert.deepEqual(plays.map((play) => play.id), ['first', 'second']);
});

test('play context uses the source tick rate and only reports known headshot counts', () => {
  const [play] = planToPlays('job-1', { demo: { tickrate: 128 }, segments: [
    { id: 'r1', round: 1, tick_start: 1280, tick_end: 2816, kills: [{ weapon: 'ak47', headshot: true }, { weapon: 'ak47', headshot: false }] },
  ] });
  assert.equal(play.startSeconds, 10);
  assert.equal(play.endSeconds, 22);
  assert.equal(play.headshots, 1);
  const [unknown] = planToPlays('job-1', { segments: [
    { id: 'r1', round: 1, tick_start: 1280, tick_end: 2816, kills: [{ weapon: 'ak47' }] },
  ] });
  assert.equal(unknown.startSeconds, undefined);
  assert.equal(unknown.headshots, undefined);
});

test('weapon ids the tactical events carry read as their in-game names', () => {
  // Every kill and grenade id in the 5kMirage and Anubis tactical documents, as internal/parser names them.
  const ids = ['ak47', 'awp', 'deagle', 'elite', 'fiveseven', 'glock', 'm4a1', 'm4a1_silencer', 'mac10', 'mp9', 'p250', 'ssg08', 'tec9',
    'usp_silencer', 'aug', 'famas', 'galilar', 'mp7', 'xm1014', 'mp5sd', 'cz75a', 'nova', 'c4', 'hegrenade', 'incgrenade', 'molotov',
    'flashbang', 'smokegrenade', 'decoy'];
  assert.deepEqual(ids.filter((id) => prettifyWeapon(id) === id), []);
  assert.equal(prettifyWeapon('usp_silencer'), 'USP-S');
  assert.equal(prettifyWeapon('m4a1'), 'M4A4');
  assert.equal(prettifyWeapon('weapon_unknown'), 'weapon_unknown');
});
