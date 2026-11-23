import test from 'node:test';
import assert from 'node:assert/strict';
import { bulkDeleteMessage, bulkStatusMessage, namesText } from './bulk.ts';
import type { CountNoun } from '../records/selection.ts';

const JUGADA: CountNoun = { one: 'jugada', many: 'jugadas', feminine: true };
const LINEUP: CountNoun = { one: 'lineup', many: 'lineups', feminine: false };

test('names are quoted and joined, and a long list is cut with a count', () => {
  assert.equal(namesText(['A']), '«A»');
  assert.equal(namesText(['A', 'B']), '«A» y «B»');
  assert.equal(namesText(['A', 'B', 'C']), '«A», «B» y «C»');
  assert.equal(namesText(['A', 'B', 'C', 'D', 'E']), '«A», «B», «C» y 2 más');
});

test('a bulk delete says how many went and names the ones that stay chosen', () => {
  assert.equal(bulkDeleteMessage({ done: 1, failed: [], noun: JUGADA }), 'Borrada 1 jugada.');
  assert.equal(bulkDeleteMessage({ done: 3, failed: [], noun: LINEUP }), 'Borrados 3 lineups.');
  assert.equal(
    bulkDeleteMessage({ done: 2, failed: ['Rush B'], noun: JUGADA }),
    'Borradas 2 jugadas. No se pudo borrar «Rush B»: sigue elegida para volver a intentarlo.',
  );
  assert.equal(
    bulkDeleteMessage({ done: 0, failed: ['Humo A', 'Molo B'], noun: LINEUP }),
    'No se pudieron borrar «Humo A» y «Molo B»: siguen elegidos para volver a intentarlo.',
  );
});

test('a bulk status change names the rows it could not change', () => {
  assert.equal(bulkStatusMessage({ done: 2, failed: [], noun: JUGADA, status: 'En uso' }), '2 jugadas pasan a En uso.');
  assert.equal(
    bulkStatusMessage({ done: 1, failed: ['Rata'], noun: JUGADA, status: 'Archivo' }),
    '1 jugada pasa a Archivo. No se pudo cambiar «Rata»: sigue elegida para volver a intentarlo.',
  );
});
