import assert from 'node:assert/strict';
import test from 'node:test';
import type { ScanSuccess } from './scan-messages.ts';
import { analysisForm, readUploadAnswer, workspaceOf, WORKSPACE_CHANGED } from './upload-analysis.ts';

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

test('the upload names the workspace the demo was queued in, and ClipMind refusing it pauses the queue', () => {
  assert.equal(workspaceOf({ user: {}, workspace: { id: 'w-a' } }), 'w-a');
  assert.equal(workspaceOf({ workspace: null }), undefined);

  const scan: ScanSuccess = {
    ok: true,
    sha256: 'sha',
    map: 'de_mirage',
    rounds: 24,
    cs2Build: 1,
    document: '{}',
    positions: new Uint8Array([1]),
    sizeBytes: 10,
    fileName: 'a.dem',
  };
  assert.equal(analysisForm({ scan, fallbackName: 'a.dem', workspace: 'w-a' }).get('workspace_id'), 'w-a');
  assert.equal(analysisForm({ scan, fallbackName: 'a.dem' }).has('workspace_id'), false);

  const moved = readUploadAnswer(409, { error: 'this demo was queued in another workspace', code: 'workspace_changed' });
  assert.deepEqual(moved, WORKSPACE_CHANGED);
  assert.equal(moved.ok ? undefined : moved.pause, 'workspace');
  const other = readUploadAnswer(409, { error: 'choose a workspace first', code: 'no_workspace' });
  assert.equal(other.ok ? 'ok' : other.pause, undefined, 'any other conflict fails only its file');
});
