import { NextResponse } from 'next/server';
import { prepareLocalUploadBody } from '@/lib/api/bounded-request-body';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { jobUrl, callOrchestrator, callOrchestratorStreamingUpload, forwardError, serviceUnavailable, UPLOAD_BODY_LIMIT_EXCEEDED } from '../../../_lib';

export const runtime = 'nodejs';

type Params = { params: Promise<{ jobId: string }> };

/** POST /api/demos/{jobId}/player-card/image: the PNG or JPG every card of the job's player shows. */
export async function POST(request: Request, { params }: Params): Promise<Response> {
  const { jobId } = await params;
  const url = jobUrl(jobId, '/player-card/image');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.startsWith('multipart/form-data;')) return NextResponse.json({ error: 'Selecciona una imagen PNG o JPG.' }, { status: 400 });
  const upload = await prepareLocalUploadBody(request, 11 * 1024 * 1024);
  if (!upload.ok) return NextResponse.json({ error: upload.error }, { status: upload.status });
  const headers: Record<string, string> = { 'Content-Type': contentType };
  if (upload.contentLength !== undefined) headers['Content-Length'] = upload.contentLength;
  const response = await callOrchestratorStreamingUpload(url, { method: 'POST', headers, body: upload.body, duplex: 'half' }, upload.exceeded);
  if (response === UPLOAD_BODY_LIMIT_EXCEEDED) return NextResponse.json({ error: 'La imagen supera los 10 MB.' }, { status: 413 });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body);
}

/** DELETE /api/demos/{jobId}/player-card/image: the cards go back to the FACEIT avatar or the initials. */
export async function DELETE(request: Request, { params }: Params): Promise<Response> {
  // The proxy skips this path for the streamed upload, so the removal checks the guard itself.
  const localError = await localAPIRequestError(request.headers, 'DELETE');
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });
  const { jobId } = await params;
  const url = jobUrl(jobId, '/player-card/image');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { method: 'DELETE' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  return new Response(null, { status: 204 });
}
