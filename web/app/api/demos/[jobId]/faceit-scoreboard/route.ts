import { localFaceitScoreboard } from '../../_local';

export const runtime = 'nodejs';

/** GET /api/demos/{jobId}/faceit-scoreboard — the FACEIT room scoreboard of a FACEIT demo. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localFaceitScoreboard(jobId);
}
