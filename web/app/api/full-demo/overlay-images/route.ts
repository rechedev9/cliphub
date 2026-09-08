import { NextResponse } from 'next/server';
import { prepareLocalUploadBody } from '@/lib/api/bounded-request-body';
import { orchestratorUrl, callOrchestratorStreamingUpload, forwardError, serviceUnavailable, UPLOAD_BODY_LIMIT_EXCEEDED } from '../../demos/_lib';

export const runtime = 'nodejs';

export async function POST(request: Request): Promise<Response> {
  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.startsWith('multipart/form-data;')) return NextResponse.json({ error: 'Selecciona una captura PNG o JPG.' }, { status: 400 });
  const upload = await prepareLocalUploadBody(request, 11 * 1024 * 1024);
  if (!upload.ok) return NextResponse.json({ error: upload.error }, { status: upload.status });
  const headers: Record<string, string> = { 'Content-Type': contentType };
  if (upload.contentLength !== undefined) headers['Content-Length'] = upload.contentLength;
  const response = await callOrchestratorStreamingUpload(`${orchestratorUrl()}/api/full-demo/overlay-images`, { method: 'POST', headers, body: upload.body, duplex: 'half' }, upload.exceeded);
  if (response === UPLOAD_BODY_LIMIT_EXCEEDED) return NextResponse.json({ error: 'La captura supera los 10 MB.' }, { status: 413 });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  return NextResponse.json(await response.json(), { status: response.status });
}
