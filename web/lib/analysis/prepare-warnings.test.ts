import assert from 'node:assert/strict';
import { test } from 'node:test';
import { prepareWarnings } from './prepare-warnings.ts';

test('the server warnings read as the line on top of Preparar', () => {
  const warnings = [
    { kind: 'stale', days: 16 },
    { kind: 'lineup', changed: 2, matches: 5 },
    { kind: 'small_sample', rounds: 12 },
  ] as const;
  assert.deepEqual(prepareWarnings(warnings, 'FUT'), [
    'La última partida de FUT fue hace 16 días: puede que hayan cambiado de playbook',
    'Alineación distinta en 2 de 5 partidas',
    'Muestra pequeña: 12 rondas',
  ]);
});

test('nothing read yet says nothing', () => {
  assert.deepEqual(prepareWarnings(null, 'FUT'), []);
  assert.deepEqual(prepareWarnings([], 'FUT'), []);
});
