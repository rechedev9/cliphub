import assert from 'node:assert/strict';
import test from 'node:test';
import { checkPin, readUploadAnswer, workspaceOf, type WorkspacePin } from './upload-analysis.ts';

test('a stored or already stored match is a success with its id', () => {
  assert.deepEqual(readUploadAnswer(201, { created: true, match: { id: 'm-1' } }), { ok: true, matchId: 'm-1', created: true });
  assert.deepEqual(readUploadAnswer(200, { created: false, match: { id: 'm-1' } }), { ok: true, matchId: 'm-1', created: false });
  assert.equal(readUploadAnswer(201, { created: true }).ok, false);
});

test('session, network and server outages pause the queue; a bad analysis fails only its file', () => {
  const pauseOf = (status: number): string | undefined => {
    const answer = readUploadAnswer(status, undefined);
    return answer.ok ? 'ok' : answer.pause;
  };
  assert.equal(pauseOf(0), 'offline');
  assert.equal(pauseOf(401), 'auth');
  assert.equal(pauseOf(503), 'unavailable');
  assert.equal(pauseOf(413), undefined);
  assert.deepEqual(readUploadAnswer(422, { error: 'map "de_x" has no usable radar calibration' }), {
    ok: false,
    error: 'ClipMind no pudo guardar esta partida.',
    detail: 'map "de_x" has no usable radar calibration',
  });
});

test('a batch keeps to the workspace its first upload saw and pauses when the session moves', () => {
  assert.equal(workspaceOf({ user: {}, workspace: { id: 'w-a' } }), 'w-a');
  assert.equal(workspaceOf({ workspace: null }), undefined);
  const pin: WorkspacePin = {};
  assert.equal(checkPin(pin, 'w-a'), undefined);
  assert.equal(checkPin(pin, undefined), undefined, 'an unreadable answer lets the upload report its own error');
  assert.deepEqual(checkPin(pin, 'w-b'), { ok: false, error: 'Has cambiado de espacio de trabajo.', pause: 'workspace' });
  assert.equal(checkPin(pin, 'w-a'), undefined);
});
