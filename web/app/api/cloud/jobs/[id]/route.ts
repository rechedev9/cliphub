import { callOrchestrator } from '../../../demos/_lib';
import { cloudJobUrl, invalidCloudRequest, relayCloud } from '../../_lib';

export const runtime = 'nodejs';

/** DELETE /api/cloud/jobs/{id}: forget a finished cloud job and its downloaded videos. */
export async function DELETE(_request: Request, { params }: { params: Promise<{ id: string }> }): Promise<Response> {
  const { id } = await params;
  const url = cloudJobUrl(id);
  if (url === null) return invalidCloudRequest('invalid cloud job id');
  return relayCloud(await callOrchestrator(url, { method: 'DELETE' }));
}
