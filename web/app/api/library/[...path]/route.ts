import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { parseControlJSONObject, prepareLocalUploadBody, readBoundedText } from '@/lib/api/bounded-request-body';
import {
  UPLOAD_BODY_LIMIT_EXCEEDED,
  callOrchestrator,
  callOrchestratorStreamingUpload,
  forwardError,
  orchestratorUrl,
  proxyStream,
  serviceUnavailable,
} from '../../demos/_lib';
import { libraryUpstreamQuery, matchLibraryRoute, pickLibraryKeys, type LibraryRoute } from '../_routes';

export const runtime = 'nodejs';

// One demo per import request (the page sends files one by one): the 700 MiB
// demo cap plus 1 MiB of multipart overhead, as for the Crear upload.
const MAX_IMPORT_REQUEST_BYTES = 701 * 1024 * 1024;

type Params = { params: Promise<{ path: string[] }> };

/** Same-origin `/api/library/*` proxy to the orchestrator, limited to the routes in `_routes.ts`. */
async function handle(request: Request, { params }: Params): Promise<Response> {
  // The import path is excluded from proxy.ts so its body is not buffered; the guard runs here for every route.
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });

  const { path } = await params;
  const route = matchLibraryRoute(request.method, path);
  if (route === undefined) return NextResponse.json({ error: 'not found' }, { status: 404 });

  const query = libraryUpstreamQuery(route, new URL(request.url).searchParams);
  const url = `${orchestratorUrl()}/api/library/${path.map(encodeURIComponent).join('/')}${query ? `?${query}` : ''}`;

  if (route.response.kind === 'stream') return proxyStream(url, 'application/octet-stream', request);

  let upstream: Response | null;
  if (route.body?.kind === 'multipart') {
    const result = await forwardImport(request, url);
    if ('refused' in result) return result.refused;
    upstream = result.upstream;
  } else {
    const init = await upstreamInit(request, route);
    if (init instanceof Response) return init;
    upstream = await callOrchestrator(url, init);
  }
  if (upstream === null) return serviceUnavailable();
  if (!upstream.ok) return forwardError(upstream);
  if (route.response.kind === 'empty') return new Response(null, { status: 204 });
  const body: unknown = await upstream.json();
  return NextResponse.json(pickLibraryKeys(body, route.response.keys), { status: upstream.status });
}

/** The upstream request for a control route: no body, or a JSON object with the route's fields only. */
async function upstreamInit(request: Request, route: LibraryRoute): Promise<RequestInit | Response> {
  if (route.body?.kind !== 'json') return { method: route.method };
  const incoming = await readBoundedText(request, route.body.maxBytes);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const parsed = parseControlJSONObject(incoming.text, route.body.fields);
  if (!parsed.ok) return NextResponse.json({ error: parsed.error }, { status: 400 });
  return {
    method: route.method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(parsed.value),
  };
}

type ImportForward = { refused: Response } | { upstream: Response | null };

/** Streams a multipart import to the orchestrator without buffering the demo. */
async function forwardImport(request: Request, url: string): Promise<ImportForward> {
  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.toLowerCase().startsWith('multipart/form-data;')) {
    return { refused: NextResponse.json({ error: 'multipart/form-data required' }, { status: 400 }) };
  }
  const upload = await prepareLocalUploadBody(request, MAX_IMPORT_REQUEST_BYTES);
  if (!upload.ok) return { refused: NextResponse.json({ error: upload.error }, { status: upload.status }) };
  const headers: Record<string, string> = { 'Content-Type': contentType };
  if (upload.contentLength !== undefined) headers['Content-Length'] = upload.contentLength;
  const res = await callOrchestratorStreamingUpload(
    url,
    { method: 'POST', headers, body: upload.body, duplex: 'half' },
    upload.exceeded,
  );
  if (res === UPLOAD_BODY_LIMIT_EXCEEDED) return { refused: NextResponse.json({ error: 'file too large' }, { status: 413 }) };
  return { upstream: res };
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
