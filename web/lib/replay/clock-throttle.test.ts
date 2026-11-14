import test from 'node:test';
import assert from 'node:assert/strict';
import { ReplayClock } from './clock.ts';
import { throttleClock } from './clock-throttle.ts';

const OPTIONS = { stepsPerSecond: 4, settleMs: 100 };

function follow(clock: ReplayClock): { heard: number[]; stop: () => void } {
  const heard: number[] = [];
  const stop = throttleClock(clock, { ...OPTIONS, onSeconds: (seconds) => heard.push(seconds) });
  return { heard, stop };
}

test('it starts from where the clock is', () => {
  const clock = new ReplayClock();
  clock.set(3.3);
  const { heard, stop } = follow(clock);
  assert.deepEqual(heard, [3.3]);
  stop();
});

test('playback is heard a few times a second, not on every frame', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const clock = new ReplayClock();
  const { heard, stop } = follow(clock);
  for (let frame = 1; frame <= 60; frame += 1) clock.set(frame / 60);
  assert.ok(heard.length <= 1 + OPTIONS.stepsPerSecond, `${heard.length} updates for one second`);
  stop();
});

test('each update carries the exact playhead, not a rounded one', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const clock = new ReplayClock();
  const { heard, stop } = follow(clock);
  clock.set(0.26);
  assert.equal(heard.at(-1), 0.26);
  stop();
});

test('a clock that stops inside a step is heard at the exact second once it settles', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const clock = new ReplayClock();
  const { heard, stop } = follow(clock);
  clock.set(0.26);
  clock.set(0.4);
  assert.equal(heard.at(-1), 0.26, 'still inside the step');
  t.mock.timers.tick(OPTIONS.settleMs);
  assert.equal(heard.at(-1), 0.4);
  stop();
});

test('a seek back inside the same step settles on the new second', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const clock = new ReplayClock();
  clock.set(10.2);
  const { heard, stop } = follow(clock);
  clock.set(10.01);
  t.mock.timers.tick(OPTIONS.settleMs);
  assert.equal(heard.at(-1), 10.01);
  stop();
});

test('stopping drops the pending settle and stops listening', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const clock = new ReplayClock();
  const { heard, stop } = follow(clock);
  clock.set(0.1);
  stop();
  t.mock.timers.tick(OPTIONS.settleMs);
  clock.set(5);
  assert.deepEqual(heard, [0]);
});
