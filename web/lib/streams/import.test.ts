import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { SERVICE_UNAVAILABLE_CODE } from '../api/types.ts';
import { isStreamURLValidationError, STREAM_INVALID_URL_MESSAGE, STREAM_OFFLINE_MESSAGE } from './plan.ts';
import {
  STREAM_IMPORT_FILE_FAIL_MESSAGE,
  STREAM_IMPORT_FILE_TOO_LARGE_MESSAGE,
  STREAM_IMPORT_FILE_UNREADABLE_MESSAGE,
  STREAM_IMPORT_URL_FAIL_MESSAGE,
  STREAM_IMPORT_YTDLP_MISSING_MESSAGE,
  STREAM_SOURCE_HOSTS,
  STREAM_URL_REQUIRED_MESSAGE,
  streamImportErrorMessage,
  streamSourceUrlError,
} from './import.ts';

function apiError(message: string, extra: { code?: string; status?: number } = {}): Error {
  return Object.assign(new Error(message), extra);
}

test('provider URLs the orchestrator accepts pass client validation', () => {
  for (const url of [
    'https://www.twitch.tv/zacketizorcs2/clip/Clutch',
    'https://clips.twitch.tv/SlugName',
    'https://m.twitch.tv/videos/123',
    'https://twitch.tv/videos/123',
    'https://www.youtube.com/watch?v=abc',
    'https://youtube.com/shorts/abc',
    'https://m.youtube.com/watch?v=abc',
    'https://music.youtube.com/watch?v=abc',
    'https://youtu.be/abc',
    'https://kick.com/streamer/clips/abc',
    'https://www.kick.com/streamer?clip=abc',
    '  https://WWW.Twitch.TV/videos/1  ',
  ]) {
    assert.equal(streamSourceUrlError(url), null, url);
  }
});

test('the host allowlist mirrors vodfetch exactly', () => {
  const vodfetch = readFileSync(new URL('../../../internal/vodfetch/vodfetch.go', import.meta.url), 'utf8');
  const block = vodfetch.match(/var allowedProviderHosts = map\[string\]struct\{\}\{([\s\S]*?)\n\}/);
  assert.ok(block, 'allowedProviderHosts not found in vodfetch.go');
  const goHosts = [...block[1].matchAll(/"([^"]+)":/g)].map((m) => m[1]);
  assert.ok(goHosts.length > 0, 'no hosts parsed from allowedProviderHosts');
  assert.deepEqual([...STREAM_SOURCE_HOSTS].sort(), goHosts.sort());
});

test('inputs the orchestrator would reject stay in the form with Spanish copy', () => {
  assert.equal(streamSourceUrlError('   '), STREAM_URL_REQUIRED_MESSAGE);
  for (const url of [
    'not a url',
    'twitch.tv/videos/1',
    'http://www.twitch.tv/videos/1',
    'ftp://www.twitch.tv/videos/1',
    'https://example.com/video.mp4',
    'https://evil-twitch.tv/videos/1',
    'https://twitch.tv.example.com/videos/1',
    'https://user:pass@www.twitch.tv/videos/1',
    'https://www.twitch.tv:8443/videos/1',
  ]) {
    assert.equal(streamSourceUrlError(url), STREAM_INVALID_URL_MESSAGE, url);
  }
  assert.match(streamSourceUrlError('https://example.com/image.png') ?? '', /archivo \.png, no a un vídeo/);
});

test('every validation message is shown beside the URL field', () => {
  for (const input of ['', 'not a url', 'https://example.com/image.png']) {
    assert.equal(isStreamURLValidationError(streamSourceUrlError(input)), true, input);
  }
});

test('URL import failures map to Spanish copy and never echo the server body', () => {
  assert.equal(
    streamImportErrorMessage(apiError('x', { code: SERVICE_UNAVAILABLE_CODE, status: 503 }), 'url'),
    STREAM_OFFLINE_MESSAGE,
  );
  assert.equal(
    streamImportErrorMessage(
      apiError('invalid source_url: unsupported twitch video url', { code: 'invalid_source_url', status: 400 }),
      'url',
    ),
    STREAM_INVALID_URL_MESSAGE,
  );
  assert.equal(
    streamImportErrorMessage(
      apiError(
        'acquiring a stream job by URL is not configured on this machine; install yt-dlp on PATH (or set ZV_YTDLP_PATH) and restart the orchestrator',
        { status: 409 },
      ),
      'url',
    ),
    STREAM_IMPORT_YTDLP_MISSING_MESSAGE,
  );
  for (const raw of ['invalid stream job JSON', 'source_url is required', 'request failed (500)', 'boom']) {
    assert.equal(streamImportErrorMessage(apiError(raw, { status: 400 }), 'url'), STREAM_IMPORT_URL_FAIL_MESSAGE, raw);
  }
  assert.equal(streamImportErrorMessage('weird', 'url'), STREAM_IMPORT_URL_FAIL_MESSAGE);
  assert.equal(STREAM_IMPORT_URL_FAIL_MESSAGE, 'No se pudo importar el vídeo. Revisa el enlace e inténtalo de nuevo.');
});

test('file import failures map to Spanish copy', () => {
  assert.equal(
    streamImportErrorMessage(apiError('file too large', { status: 413 }), 'file'),
    STREAM_IMPORT_FILE_TOO_LARGE_MESSAGE,
  );
  assert.equal(
    streamImportErrorMessage(apiError('request body too large', { status: 413 }), 'file'),
    STREAM_IMPORT_FILE_TOO_LARGE_MESSAGE,
  );
  assert.equal(
    streamImportErrorMessage(apiError('probe video: exit status 1', { status: 400 }), 'file'),
    STREAM_IMPORT_FILE_UNREADABLE_MESSAGE,
  );
  assert.equal(
    streamImportErrorMessage(apiError('missing video file: http: no such file', { status: 400 }), 'file'),
    STREAM_IMPORT_FILE_FAIL_MESSAGE,
  );
  assert.equal(
    streamImportErrorMessage(apiError('invalid source_url: x', { code: 'invalid_source_url' }), 'file'),
    STREAM_IMPORT_FILE_FAIL_MESSAGE,
  );
});
