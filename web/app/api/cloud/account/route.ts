import { callOrchestrator } from '../../demos/_lib';
import { cloudUrl, relayCloud } from '../_lib';

export const runtime = 'nodejs';

/** GET /api/cloud/account: the linked ClipHub account, its limits and the cloud queue. */
export async function GET(): Promise<Response> {
  return relayCloud(await callOrchestrator(cloudUrl('account')));
}
