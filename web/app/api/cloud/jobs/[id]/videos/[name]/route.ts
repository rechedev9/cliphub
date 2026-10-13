import { cloudVideoUrl, invalidCloudRequest, streamCloudVideo } from '../../../../_lib';

export const runtime = 'nodejs';

/** GET /api/cloud/jobs/{id}/videos/{name}: a finished cloud video, from this PC's disk. */
export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string; name: string }> },
): Promise<Response> {
  const { id, name } = await params;
  const url = cloudVideoUrl(id, name);
  if (url === null) return invalidCloudRequest('invalid cloud video');
  return streamCloudVideo(url, request);
}
