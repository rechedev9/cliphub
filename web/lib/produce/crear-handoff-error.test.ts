import test from 'node:test';
import assert from 'node:assert/strict';
import { LibraryApiError } from '../api/library.ts';
import { SHARED_MATCH_REASON } from '../library/partidas.ts';
import { crearHandoffError } from './crear-handoff-error.ts';

test('a demo whose .dem is gone says so and how to bring it back', () => {
  const { description } = crearHandoffError(new LibraryApiError('the library demo file is no longer stored', 409, 'job_demo_missing'));
  assert.match(description, /ya no está en este PC/);
  assert.match(description, /importar/);
});

test('a shared match explains that it came without its demo', () => {
  assert.equal(crearHandoffError(new LibraryApiError('shared', 409, 'shared_match')).description, SHARED_MATCH_REASON);
});

test('a dead service and a failed fetch both point at ClipHub being closed', () => {
  assert.match(crearHandoffError(new LibraryApiError('offline', 503, 'service_unavailable')).description, /no responde/);
  assert.match(crearHandoffError(new TypeError('Failed to fetch')).description, /no responde/);
});

test('a demo gone from the library and any other failure keep a next step', () => {
  assert.match(crearHandoffError(new LibraryApiError('not found', 404)).description, /biblioteca/);
  assert.match(crearHandoffError(new Error('boom')).description, /Vuelve a intentarlo/);
});
