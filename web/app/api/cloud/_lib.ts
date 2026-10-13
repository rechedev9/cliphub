// Node-resolvable specifiers, as in ../demos/_lib.ts, so node:test can import this module.
import { NextResponse } from 'next/server.js';
import { MAX_RENDER_CONTROL_BODY_BYTES, readBoundedText, type RequestBodySource } from '../../../lib/api/bounded-request-body.ts';
import { callOrchestrator, forwardError, orchestratorUrl, proxyStream, serviceUnavailable } from '../demos/_lib.ts';

const UUID_RE = /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;
/** The artifact name pattern of the worker API, limited to videos. */
const VIDEO_NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.mp4$/;

const NO_STORE = { 'cache-control': 'no-store' } as const;

/** Upstream URL of a fixed `/api/cloud/*` resource on the local orchestrator. */
export function cloudUrl(resource: 'account' | 'link' | 'jobs'): string {
  return `${orchestratorUrl()}/api/cloud/${resource}`;
}

/** Upstream URL of one cloud job; null unless the id is a UUID, so no path reaches upstream unchecked. */
export function cloudJobUrl(id: string, suffix: '' | '/cancel' = ''): string | null {
  return UUID_RE.test(id) ? `${cloudUrl('jobs')}/${id}${suffix}` : null;
}

/** Upstream URL of a downloaded cloud video; null for a bad id or anything but a plain mp4 name. */
export function cloudVideoUrl(id: string, name: string): string | null {
  if (!UUID_RE.test(id) || !VIDEO_NAME_RE.test(name)) return null;
  return `${cloudUrl('jobs')}/${id}/videos/${name}`;
}

export function invalidCloudRequest(error: string): Response {
  return NextResponse.json({ error }, { status: 400 });
}

/** Relays an orchestrator answer: its status and JSON on success, a normalized `{ error, code }` otherwise. */
export async function relayCloud(res: Response | null): Promise<Response> {
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  if (res.status === 204) return new Response(null, { status: 204 });
  let body: unknown;
  try {
    body = await res.json();
  } catch {
    return NextResponse.json({ error: 'upstream error' }, { status: 502 });
  }
  return NextResponse.json(body, { status: res.status, headers: NO_STORE });
}

/** Forwards a bounded JSON control body to the orchestrator and relays the answer. */
export async function forwardCloudJSON(request: RequestBodySource, url: string): Promise<Response> {
  const incoming = await readBoundedText(request, MAX_RENDER_CONTROL_BODY_BYTES);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  return relayCloud(
    await callOrchestrator(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: incoming.text || '{}',
    }),
  );
}

/** Streams a cloud video from this PC's disk, passing Range through so the player can seek. */
export function streamCloudVideo(url: string, request: Request): Promise<Response> {
  return proxyStream(url, 'video/mp4', request);
}
