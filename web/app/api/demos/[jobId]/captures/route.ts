import { localDeleteJobCaptures } from '../../_local';

export const runtime = 'nodejs';

/** DELETE /api/demos/{jobId}/captures: free a rendered long video's round captures; 409 busy or unrendered, 404 unknown. */
export async function DELETE(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  return localDeleteJobCaptures(jobId);
}
