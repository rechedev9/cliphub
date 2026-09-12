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

  const error = new TypeError('broken', { cause: new Error('underlying renderer failure') });
  recordRendererError('route.error', error);
  recordRendererSpan('navigation.load', 125);
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(values.length, 2);
  const recorded = values[0] as { kind: string; name: string; message: string };
  assert.equal(recorded.kind, 'error');
  assert.equal(recorded.name, 'route.error');
  assert.match(recorded.message, /TypeError: broken/);
  assert.match(recorded.message, /Caused by: Error: underlying renderer failure/);
  assert.ok(recorded.message.includes('desktop-telemetry.test.ts'), 'keep the stack for the main-process redactor');
  assert.deepEqual(values[1], { kind: 'span', name: 'navigation.load', durationMS: 125 });
});

test('is a no-op outside Electron', () => {
  recordRendererError('route.error', new Error('broken'));
  recordRendererSpan('navigation.load', Number.NaN);
});
