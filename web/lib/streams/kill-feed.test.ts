import test from 'node:test';
import assert from 'node:assert/strict';
import { killFeedDefaultPosition } from './kill-feed.ts';

const full = { x: 0, y: 0, width: 1, height: 1 };

test('the default kill feed position matches the render below the HUD', () => {
  // The Go render puts it at y 933 and 276 of 1920 for a 1080p source.
  const stacked = killFeedDefaultPosition({ variant: 'streamer-vertical-stack-40-60', gameplayCrop: full, width: 1920, height: 1080 });
  assert.equal(stacked, 933 / 1920);
  const fullFrame = killFeedDefaultPosition({ variant: 'streamer-fullframe-nocam', gameplayCrop: full, width: 1280, height: 720 });
  assert.equal(fullFrame, 276 / 1920);
  assert.equal(killFeedDefaultPosition({ variant: 'streamer-fullframe-nocam', gameplayCrop: full, width: 0, height: 0 }), null);
});
