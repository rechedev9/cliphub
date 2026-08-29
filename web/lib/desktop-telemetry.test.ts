import assert from 'node:assert/strict';
import test from 'node:test';
import { recordRendererError, recordRendererSpan } from './desktop-telemetry.ts';

test('sends fixed renderer events through the desktop bridge', async (t) => {
  const values: unknown[] = [];
  Object.defineProperty(globalThis, 'cliphubTelemetry', {
    configurable: true,
    value: {
      recordError: async (value: unknown) => { values.push(value); },
      recordSpan: async (value: unknown) => { values.push(value); },
    },
  });
  t.after(() => { Reflect.deleteProperty(globalThis, 'cliphubTelemetry'); });

  recordRendererError('route.error', new TypeError('broken'));
  recordRendererSpan('navigation.load', 125);
  await new Promise((resolve) => setImmediate(resolve));

  assert.deepEqual(values, [
    { kind: 'error', name: 'route.error' },
    { kind: 'span', name: 'navigation.load', durationMS: 125 },
  ]);
});

test('is a no-op outside Electron', () => {
  recordRendererError('route.error', new Error('broken'));
  recordRendererSpan('navigation.load', Number.NaN);
});
