import assert from 'node:assert/strict';
import test from 'node:test';
import { RendererErrorGuard, rendererErrorKey, RENDERER_ERROR_REPEAT_MS, RENDERER_ERRORS_PER_SESSION } from './renderer-error-guard.ts';

test('an identical message goes out once per minute', () => {
  let now = 1_000;
  const guard = new RendererErrorGuard({ now: () => now });
  assert.equal(guard.allow('TypeError: x is undefined'), true);
  now += 10;
  assert.equal(guard.allow('TypeError: x is undefined'), false);
  assert.equal(guard.allow('RangeError: other'), true, 'a different message is not throttled');
  now += RENDERER_ERROR_REPEAT_MS - 11;
  assert.equal(guard.allow('TypeError: x is undefined'), false);
  now += 1;
  assert.equal(guard.allow('TypeError: x is undefined'), true);
});

test('a session sends at most twenty errors', () => {
  let now = 0;
  const guard = new RendererErrorGuard({ now: () => now });
  let sent = 0;
  for (let index = 0; index < 50; index++) {
    now += RENDERER_ERROR_REPEAT_MS;
    if (guard.allow(`Error: failure ${index % 3}`)) sent++;
  }
  assert.equal(RENDERER_ERRORS_PER_SESSION, 20);
  assert.equal(sent, 20);
});

test('a render loop of one error sends one event', () => {
  const guard = new RendererErrorGuard({ now: () => 5 });
  const key = rendererErrorKey(new TypeError('Cannot read properties of undefined'));
  const sent = Array.from({ length: 500 }, () => guard.allow(key)).filter(Boolean).length;
  assert.equal(sent, 1);
  assert.equal(key, 'TypeError: Cannot read properties of undefined');
});
