import { localBatchStatus } from '../_local';

export const runtime = 'nodejs';

/**
 * GET /api/demos/batch-status?items=<jobId>:<variant>,… — every active reel's
 * job status and render state in one round trip through the same-origin
 * server boundary.
 */
export async function GET(request: Request): Promise<Response> {
  return localBatchStatus(new URL(request.url).searchParams.get('items'));
}
