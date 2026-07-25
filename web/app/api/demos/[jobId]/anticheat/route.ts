import { localAnticheat, localStartAnticheat } from '../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/anticheat — proxy the stored CheaterDetect analysis. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localAnticheat(jobId);
}

/** POST /api/demos/{jobId}/anticheat — queue the CheaterDetect analysis pass. */
export async function POST(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localStartAnticheat(jobId);
}
