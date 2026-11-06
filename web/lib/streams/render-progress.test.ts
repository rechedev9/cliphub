import test from 'node:test';
import assert from 'node:assert/strict';
import { streamRenderProgressView } from './render-progress.ts';

const variant = 'streamer-fullframe-nocam';

test('a running render shows a whole percent and the clip it encodes', () => {
  assert.deepEqual(streamRenderProgressView({ variant, percent: 42.4, clip: 2, clips: 4 }), {
    percent: 42,
    clip: 2,
    clips: 4,
    clipLabel: 'Short 2 de 4',
  });
});

test('out-of-range values are clamped instead of shown', () => {
  assert.deepEqual(streamRenderProgressView({ variant, percent: 130, clip: 9, clips: 3 }), {
    percent: 100,
    clip: 3,
    clips: 3,
    clipLabel: 'Short 3 de 3',
  });
  assert.deepEqual(streamRenderProgressView({ variant, percent: -5, clip: 0, clips: 1 }), {
    percent: 0,
    clip: 1,
    clips: 1,
    clipLabel: 'Short 1 de 1',
  });
});

test('no progress, or progress without clips, keeps the bar indeterminate', () => {
  assert.equal(streamRenderProgressView(undefined), null);
  assert.equal(streamRenderProgressView({ variant, percent: 10, clip: 1, clips: 0 }), null);
  assert.equal(streamRenderProgressView({ variant, percent: Number.NaN, clip: 1, clips: 2 }), null);
});
