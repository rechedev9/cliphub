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
