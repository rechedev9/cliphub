import test from 'node:test';
import assert from 'node:assert/strict';
import { fitWindowStateToWorkAreas, validateWindowState } from './window-state.ts';

const FALLBACK = { bounds: { width: 1280, height: 900 }, isMaximized: false };

test('accepts a full valid state with position and maximize flag', () => {
  assert.deepEqual(validateWindowState({ width: 1600, height: 1000, x: 20, y: 40, isMaximized: true }), {
    bounds: { width: 1600, height: 1000, x: 20, y: 40 },
    isMaximized: true,
  });
});

test('accepts valid dimensions without a saved position', () => {
  assert.deepEqual(validateWindowState({ width: 1024, height: 768 }), {
    bounds: { width: 1024, height: 768 },
    isMaximized: false,
  });
});

test('drops x/y unless both are finite numbers', () => {
  assert.deepEqual(validateWindowState({ width: 1024, height: 768, x: 10 }), {
    bounds: { width: 1024, height: 768 },
    isMaximized: false,
  });
  assert.deepEqual(validateWindowState({ width: 1024, height: 768, x: 10, y: Infinity }), {
    bounds: { width: 1024, height: 768 },
    isMaximized: false,
  });
});

test('falls back on missing, non-finite, implausibly small or corrupt state', () => {
  const cases: Array<{ name: string; input: unknown }> = [
    { name: 'missing width', input: { height: 900 } },
    { name: 'missing height', input: { width: 1280 } },
    { name: 'empty object', input: {} },
    { name: 'NaN width', input: { width: NaN, height: 900 } },
    { name: 'infinite height', input: { width: 1280, height: Infinity } },
    { name: 'string dimensions', input: { width: '1280', height: '900' } },
    { name: 'width below minimum', input: { width: 799, height: 900 } },
    { name: 'height below minimum', input: { width: 1280, height: 599 } },
    { name: 'null', input: null },
    { name: 'undefined', input: undefined },
    { name: 'number', input: 42 },
    { name: 'string', input: 'nope' },
    { name: 'array', input: [1, 2, 3] },
  ];
  for (const tc of cases) {
    assert.deepEqual(validateWindowState(tc.input), FALLBACK, tc.name);
  }
});

test('coerces a non-boolean isMaximized to false', () => {
  assert.deepEqual(validateWindowState({ width: 1280, height: 900, isMaximized: 'yes' }), {
    bounds: { width: 1280, height: 900 },
    isMaximized: false,
  });
});

test('moves an off-screen saved window onto the nearest current display', () => {
  assert.deepEqual(
    fitWindowStateToWorkAreas(
      {
        bounds: { width: 1280, height: 900, x: 32767, y: 32767 },
        isMaximized: false,
      },
      [{ x: 0, y: 0, width: 1920, height: 1040 }],
    ),
    {
      bounds: { width: 1280, height: 900, x: 640, y: 140 },
      isMaximized: false,
    },
  );
});

test('preserves valid negative coordinates on a real left-hand display', () => {
  assert.deepEqual(
    fitWindowStateToWorkAreas(
      {
        bounds: { width: 1200, height: 800, x: -1800, y: 100 },
        isMaximized: true,
      },
      [
        { x: 0, y: 0, width: 1920, height: 1040 },
        { x: -1920, y: 0, width: 1920, height: 1040 },
      ],
    ),
    {
      bounds: { width: 1200, height: 800, x: -1800, y: 100 },
      isMaximized: true,
    },
  );
});

test('shrinks oversized saved bounds to the selected work area', () => {
  assert.deepEqual(
    fitWindowStateToWorkAreas(
      {
        bounds: { width: 2560, height: 1440, x: 100, y: 100 },
        isMaximized: false,
      },
      [{ x: 0, y: 0, width: 1366, height: 728 }],
    ),
    {
      bounds: { width: 1366, height: 728, x: 0, y: 0 },
      isMaximized: false,
    },
  );
});
