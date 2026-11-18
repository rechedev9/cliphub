import test from 'node:test';
import assert from 'node:assert/strict';
import * as path from 'node:path';
import { parseOverlayRenderRequest } from './overlay-render-request.ts';

test('offline overlay request requires absolute files and exact delivery dimensions', () => {
  const valid = { html_path: path.resolve('overlay.html'), output_path: path.resolve('overlay.png'), width: 1920, height: 1080 };
  assert.deepEqual(parseOverlayRenderRequest(valid), valid);
  for (const invalid of [null, {}, { ...valid, html_path: 'relative.html' }, { ...valid, html_path: 'https://example.com/overlay.html' }, { ...valid, output_path: path.resolve('video.mp4') }, { ...valid, height: 1032 }]) {
    assert.throws(() => parseOverlayRenderRequest(invalid));
  }
});

test('offline overlay request admits the 9:16 Short frame and no other size', () => {
  const portrait = { html_path: path.resolve('card.html'), output_path: path.resolve('card.png'), width: 1080, height: 1920 };
  assert.deepEqual(parseOverlayRenderRequest(portrait), portrait);
  for (const size of [{ width: 1080, height: 1080 }, { width: 1920, height: 1920 }, { width: 720, height: 1280 }, { width: '1080', height: '1920' }]) {
    assert.throws(() => parseOverlayRenderRequest({ ...portrait, ...size }));
  }
});
