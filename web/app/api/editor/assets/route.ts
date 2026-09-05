import { NextResponse } from 'next/server';
import { prepareLocalUploadBody } from '@/lib/api/bounded-request-body';
import { publicEditorAsset } from '@/lib/api/public-projections';
import { orchestratorUrl, callOrchestratorStreamingUpload, forwardError, serviceUnavailable, UPLOAD_BODY_LIMIT_EXCEEDED } from '../../demos/_lib';

export const runtime = 'nodejs';

/** Upload through the existing editor asset repository; never buffer a video in Next. */
export async function POST(request: Request): Promise<Response> {
  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.startsWith('multipart/form-data;')) return NextResponse.json({ code: 'invalid_asset', error: 'multipart media required' }, { status: 400 });
  const upload = await prepareLocalUploadBody(request, 2 * 1024 * 1024 * 1024);
  if (!upload.ok) return NextResponse.json({ code: 'invalid_asset', error: upload.error }, { status: upload.status });
  const headers: Record<string, string> = { 'Content-Type': contentType };
  if (upload.contentLength !== undefined) headers['Content-Length'] = upload.contentLength;
  const response = await callOrchestratorStreamingUpload(`${orchestratorUrl()}/api/editor/assets`, {
    method: 'POST', headers, body: upload.body, duplex: 'half',
  }, upload.exceeded);
  if (response === UPLOAD_BODY_LIMIT_EXCEEDED) return NextResponse.json({ code: 'asset_too_large', error: 'file too large' }, { status: 413 });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(publicEditorAsset(body), { status: response.status });
}
