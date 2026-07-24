import { localAnticheatDossier } from '../../../../_local';

export const runtime = 'nodejs';

/**
 * GET /api/demos/{jobId}/anticheat/dossier/{steamId} — proxy the evidence pack
 * for one screened player. It is material for a report the user files
 * themselves; nothing here submits anything anywhere.
 */
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ jobId: string; steamId: string }> },
): Promise<Response> {
  const { jobId, steamId } = await params;
  return localAnticheatDossier(jobId, steamId);
}
