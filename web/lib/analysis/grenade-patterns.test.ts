import assert from 'node:assert/strict';
import test from 'node:test';
import type { TeamGrenade } from '../api/grenades.ts';
import type { RadarCalibration } from '../api/tactical.ts';
import {
  clusterGrenades,
  dragWorldBox,
  grenadeInBox,
  grenadeTracer,
  nearestCallout,
  nearestTracer,
  spotReplayRefs,
  throwTimeBins,
} from './grenade-patterns.ts';
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
    ...over,
  };
}

test('a drag box maps back to the world and selects by landing or by throw', () => {
  // Rendered at native size: world x = px * scale + pos_x, y = pos_y - py * scale.
  const box = dragWorldBox(CAL, 1024, { x: 700, y: 400 }, { x: 600, y: 300 });
  assert.deepEqual(box, { minX: 0, minY: 0, maxX: 500, maxY: 500 });
  const grenade = throwAt({ origin: [-100, 0, 0], landing: [250, 250, 0] });
  assert.equal(grenadeInBox(grenade, box, 'landing'), true);
  assert.equal(grenadeInBox(grenade, box, 'throw'), false);
  assert.equal(grenadeInBox(throwAt({ origin: null }), box, 'throw'), false);
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

test('repeated throws group into spots, most thrown first', () => {
  const spots = clusterGrenades([
    throwAt({ round: 1, thrower_steamid64: '1', thrower_name: 'ana', round_won: true }),
    throwAt({ kind: 'flash', round: 1 }),
    throwAt({ round: 2, origin: [40, 30, 0], landing: [560, 480, 0], thrower_steamid64: '2', thrower_name: 'bo', round_won: false }),
    throwAt({ round: 3, origin: null, landing: [520, 510, 0], round_won: true }),
    throwAt({ round: 4, origin: [900, 0, 0] }),
  ]);
  assert.equal(spots.length, 3);
  const [main] = spots;
  assert.equal(main?.grenades.length, 3);
  assert.equal(main?.rounds, 3);
  assert.equal(main?.roundsWon, 2);
  assert.deepEqual(main?.throwers.map((entry) => [entry.name, entry.count]), [['ana', 2], ['bo', 1]]);
  assert.deepEqual(main?.origin, [20, 15]);
});

test('throw times fill fixed bins and late throws land in the last one', () => {
  const bins = throwTimeBins([throwAt({ seconds: 0 }), throwAt({ seconds: 4.9 }), throwAt({ seconds: 12 }), throwAt({ seconds: 300 })], 5, 20);
  assert.deepEqual(bins.map((bin) => bin.count), [2, 0, 1, 1]);
  assert.deepEqual([bins[3]?.start, bins[3]?.end], [15, 20]);
});

test('a spot replays each round once, from just before its first throw', () => {
  const refs = spotReplayRefs([throwAt({ throw_tick: 2000 }), throwAt({ throw_tick: 1000 }), throwAt({ round: 2, throw_tick: null, tick: 100 })]);
  assert.deepEqual(refs, [
    { matchId: MATCH, round: 1, tick: 808 },
    { matchId: MATCH, round: 2, tick: 0 },
  ]);
});

test('callouts name the nearest place within reach', () => {
  const callouts: { name: string; center: [number, number] }[] = [
    { name: 'Ventana', center: [0, 0] },
    { name: 'Rampa', center: [400, 0] },
  ];
  assert.equal(nearestCallout(callouts, [350, 10]), 'Rampa');
  assert.equal(nearestCallout(callouts, [5000, 0]), null);
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
  const [spot] = clusterGrenades([
    throwAt({ origin: [0, 0, 0], view: [1, 1] }),
    throwAt({ round: 2, origin: [60, 0, 0], view: [2, 2] }),
    throwAt({ round: 3, origin: [30, 0, 0], view: [3, 3] }),
  ]);
  assert.ok(spot);
  assert.deepEqual(representativeThrow(spot)?.view, [3, 3]);
  const many = Array.from({ length: 25 }, () => ({ spot, label: 'Humo' }));
  assert.equal(practiceSpotsForSpots(many).length, 20);
  const [old] = clusterGrenades([throwAt({ origin: null })]);
  assert.ok(old);
  assert.deepEqual(practiceSpotsForSpots([{ spot: old, label: 'Humo' }]), []);
});
