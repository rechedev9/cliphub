import { STREAM_RENDER_ERROR_CODE, type StreamRenderState } from './api/streams.ts';

/**
 * Whether a failed render can be retried from the existing editor state. The
 * render state code, rather than its translated message, is the durable
 * contract between the worker and Studio.
 */
export function streamRenderCanRetry(state: StreamRenderState | null): boolean {
  return state?.status === 'failed' && (
    state.published === true ||
    state.error_code === STREAM_RENDER_ERROR_CODE.superseded
  );
}
