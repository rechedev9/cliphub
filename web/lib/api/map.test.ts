import assert from 'node:assert/strict';
import test from 'node:test';
import { deathMomentsToPlays, planPovPlayers, planToPlays, prettifyWeapon } from './map.ts';

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

test('deathMomentsToPlays keeps match order and says how the player died', () => {
  const plays = deathMomentsToPlays('job-1', {
    moments: [
      { segment_id: 'death-001', round: 3, time_start_seconds: 90, time_end_seconds: 99, death: { killer: 'enemy', weapon: 'awp', headshot: true } },
      { segment_id: 'death-002', round: 7, death: {} },
    ],
  });
  assert.deepEqual(plays.map((play) => play.id), ['death-001', 'death-002']);
  assert.deepEqual(plays[0].death, { killer: 'enemy', weapon: 'AWP', headshot: true });
  assert.equal(plays[0].endSeconds, 99);
  assert.equal(plays[1].startSeconds, undefined);
  assert.equal(plays[1].kills, 0);
});

test('planToPlays marks plays the recorder cannot replay from the victim', () => {
  const kill = (steamid64: string) => ({ victim: { steamid64 } });
  const plays = planToPlays('job-1', {
    demo: { tickrate: 64, duration_ticks: 50000 },
    target: { steamid64: '76561198148986856' },
    segments: [
      { id: 'player', round: 1, tick_start: 100, tick_end: 900, kills: [kill('76561198000000001')] },
      { id: 'bot', round: 2, tick_start: 1000, tick_end: 1900, kills: [kill('0')] },
      { id: 'end', round: 3, tick_start: 49000, tick_end: 49990, kills: [kill('76561198000000001')] },
    ],
  });
  assert.deepEqual(plays.map((play) => play.victimReplay), [true, false, false]);
});

test('planToPlays reads the side and the clutch the filters use', () => {
  const plays = planToPlays('job-1', {
    segments: [
      { id: 'kill', round: 2, tick_start: 0, tick_end: 64, kills: [{ weapon: 'weapon_AWP', clutch_vs: 2, killer: { team_at_kill: 'CT' } }, { weapon: 'awp', clutch_vs: 3 }] },
      { id: 'smoke', round: 14, tick_start: 100, tick_end: 164, utility: [{ thrower: { team_at_kill: 'T' } }] },
      { id: 'spectator', round: 3, tick_start: 200, tick_end: 264, kills: [{ weapon: 'ak47', killer: { team_at_kill: 'SPEC' } }] },
    ],
  });
  assert.deepEqual(plays.map((play) => [play.id, play.side, play.clutchVs, play.weapons]), [
    ['kill', 'CT', 3, ['awp']],
    ['smoke', 'T', undefined, []],
    ['spectator', undefined, undefined, ['ak47']],
  ]);
});

test('planToPlays names the player of each segment only in a multi-POV plan', () => {
  const target = { steamid64: '76561198000000001', name_in_demo: 'ropz' };
  const mate = { steamid64: '76561198000000002', name_in_demo: 'b1t' };
  const plan = {
    target,
    segments: [
      { id: 'seg-001', round: 1, tick_start: 0, tick_end: 64, pov: target },
      { id: 'seg-002', round: 2, tick_start: 640, tick_end: 704, pov: mate },
    ],
  };
  assert.deepEqual(planPovPlayers(plan), [{ steamId: target.steamid64, name: 'ropz' }, { steamId: mate.steamid64, name: 'b1t' }]);
  assert.deepEqual(planToPlays('job', plan).map((p) => p.player?.name), ['ropz', 'b1t']);
  const single = { target, segments: [{ id: 'seg-001', round: 1, tick_start: 0, tick_end: 64 }] };
  assert.deepEqual(planPovPlayers(single), []);
  assert.equal(planToPlays('job', single)[0].player, undefined);
});
