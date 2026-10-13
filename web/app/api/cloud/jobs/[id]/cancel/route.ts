import { callOrchestrator } from '../../../../demos/_lib';
import { cloudJobUrl, invalidCloudRequest, relayCloud } from '../../../_lib';

export const runtime = 'nodejs';

/** POST /api/cloud/jobs/{id}/cancel: cancel a cloud job that has not finished. */
export async function POST(_request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = cloudJobUrl(id, '/cancel');
  if (url === null) return invalidCloudRequest('invalid cloud job id');
  return relayCloud(await callOrchestrator(url, { method: 'POST' }));
}
