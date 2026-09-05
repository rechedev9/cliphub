import assert from 'node:assert/strict';
import test from 'node:test';
import { claimMediaPlayback } from './media-playback-owner.ts';

test('claim pauses the previous owner and matching release clears ownership', () => {
  const first = {};
  const second = {};
  let firstPauses = 0;
  let secondPauses = 0;

  const releaseFirst = claimMediaPlayback(first, () => {
    firstPauses += 1;
  });
  const releaseSecond = claimMediaPlayback(second, () => {
    secondPauses += 1;
  });
  releaseFirst();
  const releaseThird = claimMediaPlayback({}, () => undefined);

  assert.equal(firstPauses, 1);
  assert.equal(secondPauses, 1);
  releaseSecond();
  releaseThird();
});

test('reclaiming with the same owner does not pause itself', () => {
  const owner = {};
  let pauses = 0;
  const release = claimMediaPlayback(owner, () => {
    pauses += 1;
  });
  const latestRelease = claimMediaPlayback(owner, () => {
    pauses += 1;
  });

  assert.equal(pauses, 0);
  release();
  latestRelease();
});
