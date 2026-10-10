import { callOrchestrator } from '../../demos/_lib';
import { cloudUrl, forwardCloudJSON, relayCloud } from '../_lib';

export const runtime = 'nodejs';

/** POST /api/cloud/link: start linking this PC; the answer carries the code to confirm in the browser. */
export async function POST(request: Request): Promise<Response> {
  return forwardCloudJSON(request, cloudUrl('link'));
}

/** DELETE /api/cloud/link: unlink this PC and drop its device token. */
export async function DELETE(): Promise<Response> {
  return relayCloud(await callOrchestrator(cloudUrl('link'), { method: 'DELETE' }));
}
