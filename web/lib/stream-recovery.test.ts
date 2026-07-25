import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { STREAM_RENDER_ERROR_CODE, type StreamRenderState } from './api/streams.ts';
import { streamRenderCanRetry } from './stream-recovery.ts';

describe('streamRenderCanRetry', () => {
  it('keeps superseded and already-published failures in the editor', () => {
    const failed: StreamRenderState = { status: 'failed', videos: [] };
    assert.equal(streamRenderCanRetry({ ...failed, error_code: STREAM_RENDER_ERROR_CODE.superseded }), true);
    assert.equal(streamRenderCanRetry({ ...failed, published: true, error_code: 'ffmpeg_failed' }), true);
    assert.equal(streamRenderCanRetry({ ...failed, error_code: 'other' }), false);
    assert.equal(streamRenderCanRetry({ ...failed, status: 'rendering', error_code: STREAM_RENDER_ERROR_CODE.superseded }), false);
    assert.equal(streamRenderCanRetry(null), false);
  });
});
