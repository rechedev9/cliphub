import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalEvent } from '../api/tactical.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';
import { BOMB_SECONDS, aliveCounts, bombState, hudClock, killFeed } from './map-hud.ts';

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

test('map hud: the clock shows whole seconds', () => {
  assert.equal(hudClock(67.9), '1:07');
  assert.equal(hudClock(-12.4), '-0:12');
  assert.equal(hudClock(0), '0:00');
});
