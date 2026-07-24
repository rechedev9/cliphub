import { localTacticalAggregate } from '../../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/tactical/aggregate — proxy the filtered tendencies. */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localTacticalAggregate(jobId, new URL(request.url).searchParams);
}
