import assert from 'node:assert/strict';
import test from 'node:test';
import type { GrenadeRound, TeamGrenade } from '../api/grenades.ts';
import { denominatorText, factLabel, followUpOf, roundRows, roundsSide } from './grenade-follow-up.ts';
import { playFromGrenadeSpot } from './grenade-play.ts';
import { groupSpots, pickThrower, throwersOf } from './grenade-patterns.ts';

const MATCH = '11111111-1111-4111-8111-111111111111';

function smoke(over: Partial<TeamGrenade>): TeamGrenade {
  return {
    match_id: MATCH,
    round: 1,
    tick: 1000,
    seconds: 10,
    kind: 'smoke',
    side: 'T',
    thrower_steamid64: '1',
    thrower_name: 'ana',
    origin: [0, 0, 0],
    throw_tick: 900,
    view: [90, -10],
    landing: [500, 500, 0],
    buy: 'full',
    round_won: true,
    land_place: 'Ramp',
    throw_place: 'TSpawn',
    spot: 'smoke:ramp',
    after: { zones: [], fight: '' },
    ...over,
  };
}

function round(number: number, over: Partial<GrenadeRound> = {}): GrenadeRound {
  return { match_id: MATCH, round: number, tick: 100, side: 'T', buy: 'full', opponent_buy: 'full', won: true, category: 'execute', site: 'A', plant_seconds: null, ...over };
}

test('what follows a grenade counts each round once, every fact over the same rounds', () => {
  const rounds = [round(1), round(2, { won: false }), round(3, { category: 'default', site: 'none' }), round(4)];
  const after = followUpOf(
    [
      smoke({ round: 1, after: { zones: ['Middle', 'Firebox'], fight: 'Middle' } }),
      // A second throw in round 1 is not a second round.
      smoke({ round: 1, tick: 1200, after: { zones: ['Heaven'], fight: 'Heaven' } }),
      smoke({ round: 2, round_won: false, after: { zones: ['Firebox'], fight: 'Middle' } }),
      smoke({ round: 3, after: { zones: ['Firebox'] } }),
    ],
    rounds,
  );
  assert.equal(after.rounds, 3);
  assert.equal(after.won, 2);
  assert.deepEqual(
    after.facts.map((fact) => [fact.kind, fact.value, fact.rounds, fact.won]),
    [
      ['zone', 'Firebox', 3, 2],
      ['play', 'execute@A', 2, 1],
      ['fight', 'Middle', 2, 1],
      ['play', 'default', 1, 1],
      ['zone', 'Middle', 1, 1],
    ],
  );
  const place = (token: string): string => token.toLowerCase();
  assert.equal(factLabel({ kind: 'play', value: 'execute@A' }, place), 'Ejecución a A');
  assert.equal(factLabel({ kind: 'fight', value: 'Middle' }, place), 'Primer duelo en middle');
  assert.equal(factLabel({ kind: 'zone', value: 'Firebox' }, place), 'Uno se queda en firebox');
});

test('the denominator says the scope the bar kept', () => {
  assert.equal(denominatorText({ count: 8, total: 31, side: 'T', buy: '' }), '8 de 31 rondas en T');
  assert.equal(denominatorText({ count: 8, total: 15, side: 'T', buy: 'full' }), '8 de las 15 rondas en T con full');
  assert.equal(denominatorText({ count: 1, total: 1, side: '', buy: 'eco' }), '1 de la 1 ronda con eco');
  assert.equal(roundsSide([round(1), round(2)]), 'T');
  assert.equal(roundsSide([round(1), round(2, { side: 'CT' })]), '');
});

test('the results list the rounds with the grenade, and Invertir the rounds without it', () => {
  const rounds = [round(1), round(2), round(3)];
  const grenades = [smoke({ round: 2 }), smoke({ round: 2, tick: 1500 })];
  assert.deepEqual(
    roundRows(rounds, grenades).map((row) => [row.round.round, row.grenades.length]),
    [[2, 2]],
  );
  assert.deepEqual(
    roundRows(rounds, grenades, true).map((row) => row.round.round),
    [1, 3],
  );
});

test('a click picks one player and Shift adds one; the throwers keep their order', () => {
  assert.deepEqual(pickThrower([], 'a', false), ['a']);
  assert.deepEqual(pickThrower(['a'], 'a', false), []);
  assert.deepEqual(pickThrower(['a'], 'b', true), ['a', 'b']);
  assert.deepEqual(pickThrower(['a', 'b'], 'a', true), ['b']);
  const throwers = throwersOf([smoke({ thrower_steamid64: '2', thrower_name: 'bo' }), smoke({}), smoke({ thrower_steamid64: '2', thrower_name: 'bo' })]);
  assert.deepEqual(
    throwers.map((entry) => [entry.name, entry.throws]),
    [
      ['bo', 2],
      ['ana', 1],
    ],
  );
});

test('a grenade saved as a play keeps where it lands, what follows and its rounds from the throw', () => {
  const grenades = [smoke({ round: 1, seconds: 8, after: { zones: ['Firebox'] } }), smoke({ round: 2, seconds: 12, throw_tick: 2000, after: { zones: ['Firebox'] } })];
  const [spot] = groupSpots(grenades);
  assert.ok(spot);
  const rounds = [round(1), round(2, { site: 'B' }), round(3, { site: 'B' })];
  const play = playFromGrenadeSpot({
    teamId: 't',
    map: 'de_nuke',
    side: 'T',
    title: ' Humo a Ramp ',
    spot,
    rounds,
    followUp: followUpOf(grenades, rounds),
    place: (token) => token,
  });
  assert.equal(play.title, 'Humo a Ramp');
  assert.equal(play.origin, 'detected');
  assert.deepEqual(play.grenades, [{ kind: 'smoke', seconds: 10, pos: [500, 500, 0], callout: 'Ramp' }]);
  assert.equal(play.site, 'A');
  assert.match(play.notes, /^Granada repetida en 2 rondas; gana el 100 %\. Después: Uno se queda en Firebox \(2 de 2\)/);
  assert.deepEqual(play.examples, [
    { match_id: MATCH, round: 1, tick: 708 },
    { match_id: MATCH, round: 2, tick: 1808 },
  ]);
});
