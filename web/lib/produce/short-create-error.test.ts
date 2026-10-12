import assert from 'node:assert/strict';
import { test } from 'node:test';

import { PRODUCE_SHORT_CREATE_ERROR, PRODUCE_SHORT_CREATE_OFFLINE } from './copy.ts';
import { shortCreateErrorMessage } from './short-create-error.ts';

function apiError(message: string, extra: { code?: string; status?: number }): Error {
  return Object.assign(new Error(message), extra);
}

test('shortCreateErrorMessage maps transport and server failures to our copy', () => {
  assert.equal(shortCreateErrorMessage(apiError('internal error', { status: 500 })), PRODUCE_SHORT_CREATE_ERROR);
  assert.equal(
    shortCreateErrorMessage(apiError('down', { code: 'service_unavailable', status: 503 })),
    PRODUCE_SHORT_CREATE_OFFLINE,
  );
  assert.equal(shortCreateErrorMessage(new TypeError('Failed to fetch')), PRODUCE_SHORT_CREATE_ERROR);
  assert.equal(shortCreateErrorMessage('boom'), PRODUCE_SHORT_CREATE_ERROR);
});

test('shortCreateErrorMessage keeps the domain errors thrown on purpose', () => {
  assert.equal(shortCreateErrorMessage(new Error('Partida desconocida.')), 'Partida desconocida.');
});
