import { callOrchestrator } from '../../demos/_lib';
import { cloudUrl, forwardCloudJSON, relayCloud } from '../_lib';

export const runtime = 'nodejs';

/** GET /api/cloud/jobs: this PC's cloud jobs with their local download state. */
export async function GET(): Promise<Response> {
  return relayCloud(await callOrchestrator(cloudUrl('jobs')));
}

/** POST /api/cloud/jobs: send a parsed local match to the cloud queue. */
export async function POST(request: Request): Promise<Response> {
  return forwardCloudJSON(request, cloudUrl('jobs'));
}
