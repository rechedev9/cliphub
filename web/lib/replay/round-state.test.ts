import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalEvent, TacticalRound } from '../api/tactical.ts';
import type { WinPoint } from '../api/metrics.ts';
import { roundTimeline, timelineEvents } from '../tactical-timeline.ts';
import { alignedTick, fromLiveSeconds, liveDuration, liveEvents, liveTimeline, toLiveSeconds } from './multi-round.ts';
import { playerCards, utilityLine } from './player-cards.ts';
import { winProbabilityAt, winStripPath, winStripPoints } from './win-strip.ts';

const timeline = roundTimeline({ tick_start: 1000, tick_freeze_end: 2280, tick_end: 8680, tick_official: 9000 }, 64);
const short = roundTimeline({ tick_start: 0, tick_freeze_end: 640, tick_end: 1920, tick_official: 2000 }, 64);

function ev(kind: TacticalEvent['kind'], tick: number, actor: number, weapon?: string): TacticalEvent {
  return { kind, tick, actor_slot: actor, weapon, pos: [0, 0, 0], target_pos: [0, 0, 0] };
}

test('multi round: rounds line up at freeze end and a short round holds its end', () => {
  assert.equal(liveDuration([timeline, short]), 100);
  assert.equal(alignedTick(timeline, 0), 2280);
  assert.equal(alignedTick(timeline, 10), 2920);
  assert.equal(alignedTick(short, 50), 1920);
  assert.equal(toLiveSeconds(timeline, 5), 0);
  assert.equal(toLiveSeconds(timeline, 30), 10);
  assert.equal(fromLiveSeconds(timeline, 10), 30);
  assert.equal(liveTimeline([timeline], 64).durationSeconds, 100);
});

test('multi round: the focused events move onto the shared clock', () => {
  const events = timelineEvents(timeline, [ev('smoke', 1500, 1), ev('kill', 2920, 1)]);
  const live = liveEvents(timeline, events, 100);
  assert.equal(live.length, 1);
  assert.equal(live[0].seconds, 10);
  assert.equal(live[0].fraction, 0.1);
});

test('player cards: health, bomb, kills, grenades and weapon up to the playhead', () => {
  const round: Pick<TacticalRound, 'players' | 'tick_end'> = {
    tick_end: 6000,
    players: [
      { slot: 1, side: 'T', kills: 2, deaths: 0, assists: 0, damage: 0, equip_value: 4700, money: 800, survived: true },
      { slot: 2, side: 'CT', kills: 0, deaths: 1, assists: 0, damage: 0, equip_value: 5200, money: 300, survived: false, death_tick: 2920 },
      { slot: 3, side: 'CT', kills: 0, deaths: 0, assists: 0, damage: 0, equip_value: 1000, money: 3000, survived: true },
    ],
  };
  const events = timelineEvents(timeline, [ev('smoke', 2400, 1), ev('kill', 2920, 1, 'ak47'), ev('flash', 2950, 3), ev('kill', 5000, 1, 'deagle')]);
  const alive = TACTICAL_SAMPLE_FLAGS.alive;
  const cards = playerCards({
    round,
    samples: [
      { slot: 1, x: 0, y: 0, z: 0, yaw: 0, health: 71, flags: alive | TACTICAL_SAMPLE_FLAGS.hasBomb },
      { slot: 2, x: 0, y: 0, z: 0, yaw: 0, health: 0, flags: 0 },
    ],
    events,
    nowSeconds: (3000 - 1000) / 64,
    nowTick: 3000,
    names: new Map([[1, 'ropz']]),
  });
  const [t] = cards.t;
  assert.equal(t.name, 'ropz');
  assert.equal(t.health, 71);
  assert.equal(t.hasBomb, true);
  assert.equal(t.kills, 1);
  assert.equal(t.assists, null);
  assert.equal(t.lastWeapon, 'ak47');
  assert.equal(utilityLine(t.utility), '1 humo');
  assert.deepEqual(cards.ct.map((card) => [card.name, card.alive, card.health]), [
    ['Jugador 2', false, 0],
    ['Jugador 3', true, null],
  ]);
  assert.equal(utilityLine(cards.ct[1].utility), '1 flash');
});

test('win strip: steps sit after the freeze band and read back at a moment', () => {
  const points: WinPoint[] = [
    { tick: 0, seconds: 0, kind: 'start', alive_ct: 5, alive_t: 5, probability_ct: 0.5 },
    { tick: 0, seconds: 10, kind: 'kill', alive_ct: 5, alive_t: 4, probability_ct: 0.7 },
  ];
  assert.deepEqual(winStripPoints(points, 20, 100), [
    { fraction: 0.2, probability: 0.5 },
    { fraction: 0.3, probability: 0.7 },
  ]);
  assert.equal(winStripPath(winStripPoints(points, 0, 100), 100, 10), 'M0 5 H10 V3 H100');
  assert.equal(winProbabilityAt(points, 5), 0.5);
  assert.equal(winProbabilityAt(points, 12), 0.7);
  assert.equal(winProbabilityAt(points, -1), null);
});
