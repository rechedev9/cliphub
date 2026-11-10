import test from 'node:test';
import assert from 'node:assert/strict';
import { LibraryApiError } from './library.ts';
import { REPLAY_VIDEO_MESSAGES, replayVideoMessage } from './replay-video.ts';

// The codes internal/httpapi/library_replay_video.go and library_voice.go answer the export with.
const SERVER_CODES = ['replay_video_unavailable', 'replay_video_failed', 'invalid_request', 'voice_unsynced', 'voice_outside', 'voice_unavailable', 'no_voice', 'not_found'];

test('replay video api: every server code reads as its own Spanish line', () => {
  for (const code of SERVER_CODES) {
    const line = replayVideoMessage(new LibraryApiError('refused', 400, code));
    assert.equal(line, REPLAY_VIDEO_MESSAGES[code], code);
    assert.notEqual(line, 'No se pudo crear el MP4.', code);
  }
});

test('replay video api: a bare status or an unknown failure still gets a Spanish line', () => {
  assert.equal(replayVideoMessage(new LibraryApiError('gone', 404)), REPLAY_VIDEO_MESSAGES.not_found);
  assert.equal(replayVideoMessage(new LibraryApiError('big', 413)), REPLAY_VIDEO_MESSAGES.payload_too_large);
  assert.equal(replayVideoMessage(new LibraryApiError('odd', 400, 'brand_new_code')), 'No se pudo crear el MP4.');
  assert.equal(replayVideoMessage(new Error('network')), 'No se pudo crear el MP4.');
});
