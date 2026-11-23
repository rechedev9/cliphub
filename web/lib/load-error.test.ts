import test from 'node:test';
import assert from 'node:assert/strict';
import { LibraryApiError } from './api/library.ts';
import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { loadErrorKind, loadErrorMessage, offlineHint, offlineMessage, offlineTitle } from './load-error.ts';

test('a stopped service, a dropped fetch and a gateway error read as offline', () => {
  assert.equal(loadErrorKind(new LibraryApiError('analysis service unavailable', 503, SERVICE_UNAVAILABLE_CODE)), 'offline');
  assert.equal(loadErrorKind(new TypeError('Failed to fetch')), 'offline');
  assert.equal(loadErrorKind(new LibraryApiError('bad gateway', 502)), 'offline');
});

test('a 404 reads as not found', () => {
  assert.equal(loadErrorKind(new LibraryApiError('not found', 404)), 'not_found');
});

test('a server error, a plain error and a non-error read as a generic error', () => {
  assert.equal(loadErrorKind(new LibraryApiError('library request failed (500)', 500)), 'error');
  assert.equal(loadErrorKind(new Error('boom')), 'error');
  assert.equal(loadErrorKind(undefined), 'error');
  assert.equal(loadErrorKind('failed'), 'error');
});

test('offline copy follows the edition', () => {
  assert.match(offlineTitle('desktop'), /servicio local/);
  assert.match(offlineHint('desktop'), /ClipHub Studio/);
  assert.match(offlineMessage('desktop'), /ClipHub Studio/);
  assert.match(offlineTitle('web'), /ClipMind/);
  assert.doesNotMatch(offlineHint('web'), /Studio/);
  assert.doesNotMatch(offlineMessage('web'), /Studio/);
});

test('the message keeps the lead and never shows the raw server text', () => {
  const lead = 'No se pudo leer la partida.';
  const raw = new LibraryApiError('library request failed (500)', 500);
  const message = loadErrorMessage({ error: raw, lead, edition: 'desktop' });
  assert.ok(message.startsWith(lead));
  assert.doesNotMatch(message, /library request failed|Studio/);
  assert.match(loadErrorMessage({ error: new TypeError('Failed to fetch'), lead, edition: 'web' }), /ClipMind/);
  assert.match(loadErrorMessage({ error: new LibraryApiError('nope', 404), lead, edition: 'desktop' }), /Ya no está/);
});
