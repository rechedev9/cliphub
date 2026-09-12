import assert from 'node:assert/strict';
import test from 'node:test';
import { parseTelemetryEventRequest } from './telemetry-ipc.ts';

test('parses only allowlisted renderer event codes', () => {
  assert.deepEqual(parseTelemetryEventRequest({ kind: 'error', name: 'route.error' }), {
    kind: 'error',
    name: 'route.error',
  });
  assert.deepEqual(parseTelemetryEventRequest({
    kind: 'span',
    name: 'navigation.load',
    durationMS: 125.5,
  }), {
    kind: 'span',
    name: 'navigation.load',
    durationMS: 125.5,
  });
});

test('rejects arbitrary renderer text, labels, and event codes', () => {
  for (const value of [
    null,
    { kind: 'log', message: 'everything' },
    { kind: 'error', name: 'route.error', summary: 'C:\\demo.dem' },
    { kind: 'error', name: 'arbitrary.error' },
    { kind: 'span', name: 'navigation.load', durationMS: -1 },
    { kind: 'span', name: 'arbitrary.span', durationMS: 10 },
    { kind: 'span', name: 'navigation.load', outcome: 'secret', durationMS: 10 },
  ]) {
    assert.throws(() => parseTelemetryEventRequest(value), /invalid telemetry event/);
  }
});

test('retains the renderer cause and stack while filtering sensitive error text at IPC', () => {
  const request = parseTelemetryEventRequest({ kind: 'error', name: 'route.error',
    message: 'TypeError: render failed; token=private-renderer-token\n at render (C:\\Users\\Alice\\component.ts:42)\nCaused by: backend unavailable' });
  assert.equal(request.kind, 'error');
  if (request.kind !== 'error') throw new Error('expected an error');
  assert.match(request.message ?? '', /TypeError: render failed/);
  assert.match(request.message ?? '', /Caused by: backend unavailable/);
  assert.doesNotMatch(request.message ?? '', /Alice|private-renderer-token/);
  assert.throws(() => parseTelemetryEventRequest({ kind: 'error', name: 'route.error', message: { credentials: 'secret' } }));
  assert.throws(() => parseTelemetryEventRequest({ kind: 'error', name: 'route.error', message: 'x'.repeat(65537) }));
});
