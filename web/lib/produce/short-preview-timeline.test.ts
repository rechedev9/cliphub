import test from 'node:test';
import assert from 'node:assert/strict';
import type { EditConfig } from '../api/types.ts';
import { previewFrame, previewTimeline } from './short-preview-timeline.ts';

const EDIT: EditConfig = {
  format: 'short-9x16', killEffect: 'clean', transition: 'cut', intro: false, outro: false, hookText: false, killCounter: false,
  matchRecap: false, voiceComms: false, nativeHud: false, coverStrategy: 'generated-gameplay',
};

function at(edit: Partial<EditConfig>, t: number): ReturnType<typeof previewFrame> {
  return previewFrame(previewTimeline({ ...EDIT, ...edit }), t);
}

test('each kill effect moves the frame only around the kill, with the render strength', () => {
  const [kill = 0] = previewTimeline(EDIT).kills;
  assert.equal(at({ killEffect: 'clean' }, kill).zoom, 1);
  assert.ok(Math.abs(at({ killEffect: 'punch-in' }, kill).zoom - 1.08) < 1e-9);
  assert.equal(at({ killEffect: 'punch-in' }, kill - 0.5).zoom, 1);
  assert.deepEqual(at({ killEffect: 'freeze-flash' }, kill).flash, { color: 'white', opacity: 0.34 });
  assert.equal(at({ killEffect: 'glitch' }, kill).glitch, 12);
  assert.ok(at({ killEffect: 'shake' }, kill + 0.01).shakeX !== 0);
  assert.equal(at({ killEffect: 'velocity' }, kill).grade, true);
});

test('the transition dresses the cut into the second play', () => {
  const { cut } = previewTimeline(EDIT);
  assert.equal(at({ transition: 'cut' }, cut).flash, null);
  assert.equal(at({ transition: 'flash' }, cut).flash?.opacity, 1);
  assert.equal(at({ transition: 'dip' }, cut + 0.05).flash?.color, 'black');
  assert.ok(at({ transition: 'zoom-whip' }, cut + 0.02).zoom > 1.5);
  assert.equal(at({ transition: 'glitch' }, cut - 0.05).glitch, 10);
  assert.equal(at({}, cut).clip, 'b');
});

test('the kill counter counts up and names the multikill on the last kill unless badges do', () => {
  const { kills } = previewTimeline(EDIT);
  const [first = 0, , last = 0] = kills;
  assert.equal(at({ killCounter: true }, first + 0.1).counter?.value, '1');
  assert.equal(at({ killCounter: true }, last + 0.1).counter?.value, '3K');
  assert.equal(at({ killCounter: true, playBadges: true }, last + 0.1).counter?.value, '3');
  assert.equal(at({}, first + 0.1).counter, null);
});

test('the victim replay sits between the plays and carries no kill effect', () => {
  const timeline = previewTimeline({ ...EDIT, victimReplay: true, victimReplaySpeed: 0.5, killEffect: 'punch-in' });
  assert.ok(timeline.replay);
  const middle = (timeline.replay.start + timeline.replay.end) / 2;
  const frame = previewFrame(timeline, middle);
  assert.equal(frame.clip, 'replay');
  assert.equal(frame.zoom, 1);
  assert.equal(timeline.cut, timeline.replay.end);
});

test('the title, the opening card and the stats card follow their switches', () => {
  assert.ok(at({ hookText: true }, 0.5).hook > 0);
  assert.equal(at({ hookText: true, intro: true }, 0.5).hook, 0);
  assert.ok(at({ intro: true }, 0.5).intro > 0);
  const timeline = previewTimeline({ ...EDIT, statsCard: true });
  assert.ok(timeline.statsStart !== null);
  assert.ok(previewFrame(timeline, timeline.duration - 0.1).stats > 0);
  assert.equal(previewTimeline({ ...EDIT, statsCard: true, format: 'landscape-16x9' }).statsStart, null);
});
