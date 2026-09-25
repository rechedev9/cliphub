// Unit tests for the Partidas delete error → Spanish message mapping.
// Run: node --test delete-error.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { deleteErrorMessage, DELETE_OFFLINE_MESSAGE, DELETE_GENERIC_MESSAGE } from './delete-error.ts';

test('offline code maps to the start-your-orchestrator hint', () => {
  const err = Object.assign(new Error('analysis service unavailable'), { code: SERVICE_UNAVAILABLE_CODE });
  assert.equal(deleteErrorMessage(err), DELETE_OFFLINE_MESSAGE);
});

test('a non-offline error surfaces its message verbatim', () => {
  const cases: { name: string; err: Error }[] = [
    { name: '409 orchestrator explanation', err: new Error('Espera a que termine la captura para borrar') },
    { name: 'non-offline code', err: Object.assign(new Error('job not found'), { code: 'not_found' }) },
  ];
  for (const { name, err } of cases) {
    assert.equal(deleteErrorMessage(err), err.message, name);
  }
});

test('missing or blank message falls back to the generic retry line', () => {
  assert.equal(deleteErrorMessage(new Error('')), DELETE_GENERIC_MESSAGE);
  assert.equal(deleteErrorMessage(new Error('   ')), DELETE_GENERIC_MESSAGE);
  assert.equal(deleteErrorMessage(null), DELETE_GENERIC_MESSAGE);
  assert.equal(deleteErrorMessage({}), DELETE_GENERIC_MESSAGE);
});
