import assert from 'node:assert/strict';
import test from 'node:test';
import { recordRendererError, recordRendererSpan, rendererRoute } from './desktop-telemetry.ts';

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
  assert.match(recorded.message, /^digest=none route=unknown\n/);
  assert.match(recorded.message, /TypeError: broken/);
  assert.match(recorded.message, /Caused by: Error: underlying renderer failure/);
  assert.ok(recorded.message.includes('desktop-telemetry.test.ts'), 'keep the stack for the main-process redactor');
  assert.deepEqual(values[1], { kind: 'span', name: 'navigation.load', durationMS: 125 });
});

test('the renderer event leads with the digest and route pattern', async (t) => {
  const values: Array<{ message: string }> = [];
  Object.defineProperty(globalThis, 'cliphubTelemetry', {
    configurable: true,
    value: {
      recordError: async (value: { message: string }) => { values.push(value); },
      recordSpan: async () => {},
    },
  });
  t.after(() => { Reflect.deleteProperty(globalThis, 'cliphubTelemetry'); });

  recordRendererError('route.error', new Error('boom'), {
    digest: '2345678901',
    pathname: '/clips/3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e/publicar/clip-7',
  });
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(values[0]?.message.split('\n')[0], 'digest=2345678901 route=clips.{id}.publicar.{id}');
});

test('route patterns keep static segments and hide ids', () => {
  assert.equal(rendererRoute('/clips'), 'clips');
  assert.equal(rendererRoute('/clips/3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e/nuevo'), 'clips.{id}.nuevo');
  assert.equal(rendererRoute('/players/s1mple'), 'players.{id}');
  assert.equal(rendererRoute('/tactical/3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e?round=3'), 'tactical.{id}');
  assert.equal(rendererRoute('/'), 'root');
  assert.equal(rendererRoute(undefined), 'unknown');
});

test('is a no-op outside Electron', () => {
  recordRendererError('route.error', new Error('broken'));
  recordRendererSpan('navigation.load', Number.NaN);
});
