// The browser's only door to the ClipMind account API. Tokens never reach page
// code: login and switching write them to HttpOnly cookies here, and every other
// call carries the access token the proxy resolved for the request.
import { NextResponse, type NextRequest } from 'next/server.js';
import {
  ACCESS_HEADER,
  REFRESH_COOKIE,
  clearedCookies,
  clipMindUrl,
  parseTokenSet,
  refreshTokens,
  sessionCookies,
  type CookieSpec,
} from '../../../../lib/clipmind/session.ts';

interface Context {
  params: Promise<{ path: string[] }>;
}

const PUBLIC_POSTS: readonly string[] = ['auth/signup', 'auth/confirm', 'auth/forgot', 'auth/reset'];

/** Account routes a signed-in browser may call; the server still authorizes each one. */
const PROXIED = [
  /^workspaces(\/[A-Za-z0-9_-]+(\/(members(\/[A-Za-z0-9_-]+)?|transfer|invitations(\/[A-Za-z0-9_-]+)?))?)?$/,
  /^invitations\/accept$/,
  /^auth\/(me|sessions|tokens|export|account)$/,
  /^auth\/(sessions|tokens)\/[A-Za-z0-9_-]+$/,
];

function isSecure(request: NextRequest): boolean {
  return request.nextUrl.protocol === 'https:' || request.headers.get('x-forwarded-proto') === 'https';
}

function withCookies(response: NextResponse, specs: CookieSpec[]): NextResponse {
  for (const spec of specs) response.cookies.set(spec.name, spec.value, spec.options);
  return response;
}

function unavailable(): NextResponse {
  return NextResponse.json({ error: 'ClipMind is not reachable', code: 'unavailable' }, { status: 503 });
}

async function call(path: string, init: RequestInit): Promise<Response | undefined> {
  try {
    return await fetch(`${clipMindUrl()}/${path}`, { ...init, cache: 'no-store', redirect: 'manual' });
  } catch (error) {
    console.error(`clipmind unreachable: ${init.method ?? 'GET'} /${path}`, error);
    return undefined;
  }
}

async function relay(upstream: Response): Promise<NextResponse> {
  const contentType = upstream.headers.get('content-type') ?? 'application/json';
  const headers = new Headers({ 'Content-Type': contentType, 'Cache-Control': 'no-store' });
  const disposition = upstream.headers.get('content-disposition');
  if (disposition !== null) headers.set('Content-Disposition', disposition);
  return new NextResponse(upstream.body, { status: upstream.status, headers });
}

async function login(request: NextRequest): Promise<NextResponse> {
  const body = await request.text();
  const upstream = await call('auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body });
  if (upstream === undefined) return unavailable();
  if (!upstream.ok) return relay(upstream);
  const tokens = parseTokenSet(await upstream.json());
  if (tokens === undefined) return NextResponse.json({ error: 'unexpected answer', code: 'server_error' }, { status: 502 });
  return withCookies(NextResponse.json({ ok: true, workspace_id: tokens.workspaceId }), sessionCookies(tokens, isSecure(request)));
}

async function switchWorkspace(request: NextRequest): Promise<NextResponse> {
  const refresh = request.cookies.get(REFRESH_COOKIE)?.value;
  if (refresh === undefined) return NextResponse.json({ error: 'sign in required', code: 'unauthorized' }, { status: 401 });
  const parsed: unknown = await request.json().catch(() => undefined);
  const workspaceId = typeof parsed === 'object' && parsed !== null && 'workspace_id' in parsed && typeof parsed.workspace_id === 'string' ? parsed.workspace_id : '';
  if (workspaceId === '') return NextResponse.json({ error: 'workspace_id is required', code: 'invalid_input' }, { status: 400 });
  // The proxy may have rotated this refresh token a moment ago; the shared exchange hands back that same result.
  const current = await refreshTokens(refresh);
  const switched = current === undefined ? undefined : await refreshTokens(current.refresh, workspaceId);
  if (switched === undefined) return withCookies(NextResponse.json({ error: 'cannot switch', code: 'forbidden' }, { status: 403 }), clearedCookies(isSecure(request)));
  return withCookies(NextResponse.json({ ok: true, workspace_id: switched.workspaceId }), sessionCookies(switched, isSecure(request)));
}

async function logout(request: NextRequest): Promise<NextResponse> {
  const access = request.headers.get(ACCESS_HEADER);
  if (access !== null) await call('auth/logout', { method: 'POST', headers: { Authorization: `Bearer ${access}` } });
  return withCookies(NextResponse.json({ ok: true }), clearedCookies(isSecure(request)));
}

async function handle(request: NextRequest, context: Context): Promise<NextResponse> {
  const path = (await context.params).path.join('/');
  const method = request.method;

  if (method === 'POST' && path === 'auth/login') return login(request);
  if (method === 'POST' && path === 'auth/switch') return switchWorkspace(request);
  if (method === 'POST' && path === 'auth/logout') return logout(request);

  const isPublic = method === 'POST' && PUBLIC_POSTS.includes(path);
  if (!isPublic && !PROXIED.some((pattern) => pattern.test(path))) {
    return NextResponse.json({ error: 'not found', code: 'not_found' }, { status: 404 });
  }

  const headers: Record<string, string> = {};
  const contentType = request.headers.get('content-type');
  if (contentType !== null) headers['Content-Type'] = contentType;
  if (!isPublic) {
    const access = request.headers.get(ACCESS_HEADER);
    if (access === null) return NextResponse.json({ error: 'sign in required', code: 'unauthorized' }, { status: 401 });
    headers.Authorization = `Bearer ${access}`;
  }
  const hasBody = method !== 'GET' && method !== 'HEAD';
  const upstream = await call(`${path}${request.nextUrl.search}`, { method, headers, body: hasBody ? await request.text() : undefined });
  if (upstream === undefined) return unavailable();
  const response = await relay(upstream);
  // Deleting the account ends the sign-in with it.
  if (method === 'DELETE' && path === 'auth/account' && upstream.ok) return withCookies(response, clearedCookies(isSecure(request)));
  return response;
}

export const GET = handle;
export const POST = handle;
export const PATCH = handle;
export const DELETE = handle;
