import { localTacticalPositions } from '../../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/tactical/positions — stream the Range-capable zvpos1 blob. */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localTacticalPositions(jobId, request);
}
