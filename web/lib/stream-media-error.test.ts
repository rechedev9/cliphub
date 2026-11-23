import assert from 'node:assert/strict';
import { test } from 'node:test';
import { streamMediaFailure } from './stream-media-error.ts';

test('a media error keeps its code for telemetry and shows a Spanish sentence', () => {
  const failure = streamMediaFailure({ code: 3, message: 'PIPELINE_ERROR_DECODE' });
  assert.equal(failure.detail, 'MediaError 3: PIPELINE_ERROR_DECODE');
  assert.doesNotMatch(failure.message, /MediaError|PIPELINE/);
  assert.match(failure.message, /dañado/);
});

test('an unknown or missing media error still reads as a next step', () => {
  assert.match(streamMediaFailure({ code: 9, message: '' }).message, /Vuelve a intentar/);
  assert.match(streamMediaFailure(null).message, /Vuelve a intentar/);
  assert.equal(streamMediaFailure({ code: 2, message: '' }).detail, 'MediaError 2: no message');
});
