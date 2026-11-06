import test from 'node:test';
import assert from 'node:assert/strict';
import { acquireProgressView, formatTimeLeft } from './acquire-progress.ts';

test('no update yet keeps the indeterminate card', () => {
  assert.deepEqual(acquireProgressView(undefined), { stage: 'Descargando', detail: 'El editor se abre al terminar' });
});

test('a Twitch VOD shows fragment percent, estimated size and time left', () => {
  const view = acquireProgressView({
    downloaded_bytes: 160_391_060,
    total_bytes: 1_359_970_186,
    total_estimated: true,
    percent: 11.78,
    eta_seconds: 178,
  });
  assert.deepEqual(view, { stage: 'Descargando', percent: 11, detail: '153 MB de ~1,3 GB · quedan 3 min' });
});

test('the audio track is named, and 99,6 never reads as done', () => {
  const view = acquireProgressView({ audio_only: true, downloaded_bytes: 996, total_bytes: 1000, percent: 99.6 });
  assert.equal(view.stage, 'Descargando audio');
  assert.equal(view.percent, 99);
});

test('a live source without length shows bytes and no number', () => {
  const view = acquireProgressView({ downloaded_bytes: 5 * 1024 * 1024, eta_seconds: 30 });
  assert.deepEqual(view, { stage: 'Descargando', percent: undefined, detail: '5 MB descargados' });
});

test('a finished track switches to the preparing stage without a bar number', () => {
  assert.deepEqual(acquireProgressView({ downloaded_bytes: 10, total_bytes: 10, percent: 100 }), {
    stage: 'Preparando el vídeo',
    detail: 'El editor se abre al terminar',
  });
});

test('time left reads in minutes and hours', () => {
  assert.equal(formatTimeLeft(20), 'queda menos de 1 min');
  assert.equal(formatTimeLeft(178), 'quedan 3 min');
  assert.equal(formatTimeLeft(3900), 'quedan 1 h 05 min');
});
