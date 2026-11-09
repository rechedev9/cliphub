import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalEvent } from '../api/tactical.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';
import { BOMB_SECONDS, airborneIn, aliveCounts, bombState, hudClock, killFeed, killMarks, postPlantSpan } from './map-hud.ts';

function at(kind: TacticalEvent['kind'], seconds: number, site?: TacticalEvent['site']): TimelineEvent {
  return { event: { kind, tick: seconds * 64, site, pos: [0, 0, 0], target_pos: [0, 0, 0] }, seconds, fraction: 0 };
}

test('map hud: the bomb counts down from the plant and stops at the defuse', () => {
  const events = [at('kill', 10), at('plant', 50, 'A'), at('defuse', 80)];
  assert.deepEqual(bombState(events, 40), { kind: 'none' });
  assert.deepEqual(bombState(events, 60), { kind: 'planted', site: 'A', remaining: BOMB_SECONDS - 10 });
  assert.deepEqual(bombState(events, 85), { kind: 'defused' });
  assert.deepEqual(bombState([at('plant', 0), at('explode', 40)], 41), { kind: 'exploded' });
});

test('map hud: the post-plant band runs from the plant to the defuse, the explosion or the end of the bar', () => {
  assert.deepEqual(postPlantSpan([at('kill', 10), at('plant', 50, 'A'), at('defuse', 80)], 100), { from: 50, to: 80 });
  assert.deepEqual(postPlantSpan([at('plant', 50), at('explode', 90)], 100), { from: 50, to: 90 });
  assert.deepEqual(postPlantSpan([at('plant', 50)], 70), { from: 50, to: 70 });
  assert.equal(postPlantSpan([at('kill', 10)], 100), null);
});

test('map hud: the feed keeps the last few seconds of kills, oldest first', () => {
  const events = [at('kill', 1), at('smoke', 2), at('kill', 5), at('kill', 9)];
  assert.deepEqual(killFeed(events, 9).map((entry) => entry.seconds), [5, 9]);
  assert.deepEqual(killFeed(events, 4).map((entry) => entry.seconds), [1]);
  assert.deepEqual(killFeed(events, 30), []);
});

test('map hud: living players are counted per side', () => {
  const { alive, sideT } = TACTICAL_SAMPLE_FLAGS;
  assert.deepEqual(aliveCounts([{ flags: alive }, { flags: alive | sideT }, { flags: sideT }, { flags: alive | sideT }]), { CT: 1, T: 2 });
});

test('map hud: the feed marks how each kill was made, the air read from the killer', () => {
  const kill = { ...at('kill', 10).event, actor_slot: 3, headshot: true, through_smoke: true, attacker_blind: true };
  assert.deepEqual(
    killMarks(kill, true).map((mark) => mark.short),
    ['HS', 'Humo', 'Aire', 'Cegado'],
  );
  assert.deepEqual(killMarks(at('kill', 10).event, false), []);
  const { alive, airborne } = TACTICAL_SAMPLE_FLAGS;
  const samples = [{ slot: 3, flags: alive | airborne }, { slot: 4, flags: alive }];
  assert.equal(airborneIn(samples, 3), true);
  assert.equal(airborneIn(samples, 4), false);
  assert.equal(airborneIn(samples, undefined), false);
});

test('map hud: the clock shows whole seconds', () => {
  assert.equal(hudClock(67.9), '1:07');
  assert.equal(hudClock(-12.4), '-0:12');
  assert.equal(hudClock(0), '0:00');
});
