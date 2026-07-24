import { localTacticalStatus } from '../../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/tactical/status — proxy the analysis lifecycle state. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localTacticalStatus(jobId);
}
