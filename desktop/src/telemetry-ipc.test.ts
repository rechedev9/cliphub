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
