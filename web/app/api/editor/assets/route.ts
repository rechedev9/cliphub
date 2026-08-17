import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { prepareLocalUploadBody } from '@/lib/api/bounded-request-body';
import {
  orchestratorUrl,
  callOrchestrator,
  forwardError,
  serviceUnavailable,
  callOrchestratorStreamingUpload,
  UPLOAD_BODY_LIMIT_EXCEEDED,
} from '../_lib';

export const runtime = 'nodejs';

const MAX_UPLOAD_BYTES = 2 * 1024 * 1024 * 1024;

export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/editor/assets`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}

export async function POST(request: Request): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });
  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.includes('multipart/form-data')) {
    return NextResponse.json({ error: 'multipart video upload required' }, { status: 400 });
  }
  const upload = await prepareLocalUploadBody(request, MAX_UPLOAD_BYTES);
  if (!upload.ok) return NextResponse.json({ error: upload.error }, { status: upload.status });
  const headers: Record<string, string> = { 'Content-Type': contentType };
  if (upload.contentLength !== undefined) headers['Content-Length'] = upload.contentLength;
  const res = await callOrchestratorStreamingUpload(
    `${orchestratorUrl()}/api/editor/assets`,
    { method: 'POST', headers, body: upload.body, duplex: 'half' },
    upload.exceeded,
  );
  if (res === UPLOAD_BODY_LIMIT_EXCEEDED) {
    return NextResponse.json({ error: 'file too large' }, { status: 413 });
  }
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown, { status: res.status });
}
