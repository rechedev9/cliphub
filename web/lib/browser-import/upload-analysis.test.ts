import assert from 'node:assert/strict';
import test from 'node:test';
import { readUploadAnswer } from './upload-analysis.ts';

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
