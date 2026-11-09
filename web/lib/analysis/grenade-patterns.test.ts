import assert from 'node:assert/strict';
import test from 'node:test';
import type { TeamGrenade } from '../api/grenades.ts';
import type { RadarCalibration } from '../api/tactical.ts';
import { clockBins, dragWorldBox, grenadeInBox, grenadeTracer, groupSpots, nearestTracer, spotReplayRefs } from './grenade-patterns.ts';
import { practiceSpotFor, practiceSpotsForSpots, representativeThrow } from './practice-spots.ts';

const MATCH = '11111111-1111-4111-8111-111111111111';
const CAL: RadarCalibration = { map: 'de_mirage', source: 'overview', pos_x: -3000, pos_y: 2000, scale: 5, size: 1024 };

function throwAt(over: Partial<TeamGrenade>): TeamGrenade {
  return {
    match_id: MATCH,
    round: 1,
    tick: 1000,
    seconds: 10,
    kind: 'smoke',
    side: 'T',
    thrower_steamid64: '1',
    thrower_name: 'ana',
    origin: [0, 0, 10],
    throw_tick: 900,
    view: [90, -20],
    landing: [500, 500, 0],
    buy: 'full',
    round_won: true,
    land_place: '',
    throw_place: '',
    spot: 'smoke:a',
    after: { zones: [] },
    ...over,
  };
}

test('a drag box maps back to the world and selects by landing or by throw', () => {
  // Rendered at native size: world x = px * scale + pos_x, y = pos_y - py * scale.
  const box = dragWorldBox(CAL, 1024, { x: 700, y: 400 }, { x: 600, y: 300 });
  assert.deepEqual(box, { minX: 0, minY: 0, maxX: 500, maxY: 500 });
  const grenade = throwAt({ origin: [-100, 0, 0], landing: [250, 250, 0] });
  assert.equal(grenadeInBox({ grenade, box, area: 'landing' }), true);
  assert.equal(grenadeInBox({ grenade, box, area: 'throw' }), false);
  assert.equal(grenadeInBox({ grenade: throwAt({ origin: null }), box, area: 'throw' }), false);
});

test('tracers follow the radar transform and the nearest one is hit', () => {
  const tracer = grenadeTracer(CAL, 512, throwAt({ origin: [-3000, 2000, 0], landing: [-2000, 1000, 0] }));
  assert.deepEqual(tracer, { from: { x: 0, y: 0 }, to: { x: 100, y: 100 } });
  const landingOnly = grenadeTracer(CAL, 512, throwAt({ origin: null, landing: [-3000, 2000, 0] }));
  assert.deepEqual(landingOnly, { from: null, to: { x: 0, y: 0 } });
  assert.equal(grenadeTracer({ ...CAL, scale: 0 }, 512, throwAt({})), null);
  const tracers = [tracer, landingOnly].flatMap((entry) => (entry === null ? [] : [entry]));
  assert.equal(nearestTracer(tracers, { x: 50, y: 52 }, 6), 0);
  assert.equal(nearestTracer(tracers, { x: 300, y: 0 }, 6), -1);
});

test('throws group by the spot the server named, most rounds first', () => {
  const spots = groupSpots([
    throwAt({ round: 1, thrower_steamid64: '1', thrower_name: 'ana', round_won: true }),
    throwAt({ kind: 'flash', round: 1, spot: 'flash:b' }),
    throwAt({ round: 2, origin: [40, 30, 0], thrower_steamid64: '2', thrower_name: 'bo', round_won: false }),
    throwAt({ round: 3, origin: null, round_won: true }),
    throwAt({ round: 4, spot: '' }),
  ]);
  assert.equal(spots.length, 3);
  const [main] = spots;
  assert.equal(main?.key, 'smoke:a');
  assert.equal(main?.grenades.length, 3);
  assert.equal(main?.rounds, 3);
  assert.equal(main?.roundsWon, 2);
  assert.deepEqual(main?.throwers.map((entry) => [entry.name, entry.count]), [['ana', 2], ['bo', 1]]);
  assert.deepEqual(main?.origin, [20, 15]);
  assert.ok(spots.some((spot) => spot.key.startsWith('smoke:') && spot.key !== 'smoke:a'), 'a throw without a key stands alone');
});

test('the round clock bins stack each player and late throws land in the last bin', () => {
  const bins = clockBins([throwAt({ seconds: 0 }), throwAt({ seconds: 4.9, thrower_steamid64: '2' }), throwAt({ seconds: 12 }), throwAt({ seconds: 300 })], 5);
  assert.equal(bins.length, 23);
  assert.deepEqual([bins[0]?.count, bins[0]?.byPlayer.get('1'), bins[0]?.byPlayer.get('2')], [2, 1, 1]);
  assert.equal(bins[2]?.count, 1);
  assert.deepEqual([bins[22]?.start, bins[22]?.end, bins[22]?.count], [110, 115, 1]);
});

test('a spot replays each round once, from just before its first throw', () => {
  const refs = spotReplayRefs([throwAt({ throw_tick: 2000 }), throwAt({ throw_tick: 1000 }), throwAt({ round: 2, throw_tick: null, tick: 100 })]);
  assert.deepEqual(refs, [
    { matchId: MATCH, round: 1, tick: 808 },
    { matchId: MATCH, round: 2, tick: 0 },
  ]);
});

test('practice spots take feet position, pitch then yaw, and the side grenade', () => {
  assert.deepEqual(practiceSpotFor(throwAt({ kind: 'molotov', side: 'CT' }), 'Molotov a Banana'), {
    label: 'Molotov a Banana',
    pos: [0, 0, 10],
    ang: [-20, 90],
    grenade: 'incendiary',
  });
  assert.equal(practiceSpotFor(throwAt({ origin: null }), 'x'), null);
});

test('a group practises its most central real throw, capped at 20 spots', () => {
  const [spot] = groupSpots([
    throwAt({ origin: [0, 0, 0], view: [1, 1] }),
    throwAt({ round: 2, origin: [60, 0, 0], view: [2, 2] }),
    throwAt({ round: 3, origin: [30, 0, 0], view: [3, 3] }),
  ]);
  assert.ok(spot);
  assert.deepEqual(representativeThrow(spot)?.view, [3, 3]);
  const many = Array.from({ length: 25 }, () => ({ spot, label: 'Humo' }));
  assert.equal(practiceSpotsForSpots(many).length, 20);
  const [old] = groupSpots([throwAt({ origin: null })]);
  assert.ok(old);
  assert.deepEqual(practiceSpotsForSpots([{ spot: old, label: 'Humo' }]), []);
});

const NUKE: RadarCalibration = { map: 'de_nuke', source: 'overview', pos_x: -3453, pos_y: 2887, scale: 7, size: 1024, lower_altitude_max: -495 };

test('on a two-floor map the box selects on the floor it was drawn on', () => {
  const box = { minX: 400, minY: 400, maxX: 600, maxY: 600 };
  const lower = { calibration: NUKE, level: 'lower' as const };
  // Thrown from A site above, landing in B below: on the B radar only the landing is in.
  const fromA = throwAt({ origin: [500, 500, -400], landing: [500, 500, -700] });
  assert.equal(grenadeInBox({ grenade: fromA, box, area: 'landing', floor: lower }), true);
  assert.equal(grenadeInBox({ grenade: fromA, box, area: 'throw', floor: lower }), false);
  assert.equal(grenadeInBox({ grenade: fromA, box, area: 'throw' }), true);
});
