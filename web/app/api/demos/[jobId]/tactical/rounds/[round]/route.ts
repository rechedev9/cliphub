import { localTacticalRound } from '../../../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/tactical/rounds/{round} — proxy one round and its frames. */
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ jobId: string; round: string }> },
): Promise<Response> {
  const { jobId, round } = await params;
  return localTacticalRound(jobId, round);
}
