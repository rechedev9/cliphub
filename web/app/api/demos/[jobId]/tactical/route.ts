import { localStartTactical, localTacticalDocument } from '../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/tactical — proxy the tactical analysis document. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localTacticalDocument(jobId);
}

/** POST /api/demos/{jobId}/tactical — start the tactical analysis of a parsed demo. */
export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localStartTactical(request, jobId);
}
