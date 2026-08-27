import 'server-only';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { configuredControlPlaneURL } from '@/lib/hosted-mode';

const SESSION_COOKIE_NAME = 'cliphub_session';
const CONTROL_BODY_LIMIT = 16 * 1024;
const CONTROL_PATH_PATTERN = /^[a-z0-9][a-z0-9/-]*$/;

export async function forwardControlPlane(request: Request, namespace: 'account' | 'agent', segments: readonly string[]): Promise<Response> {
  const path = segments.join('/');
  if (!CONTROL_PATH_PATTERN.test(path)) {
    return Response.json({ code: 'invalid_path', error: 'Ruta no válida.' }, { status: 400 });
  }
  const base = configuredControlPlaneURL();
  if (base === null) {
    return Response.json(
      { code: 'service_unavailable', error: 'El servicio de cuentas no está disponible.' },
      { status: 503 },
    );
  }

  const headers: Record<string, string> = { Accept: 'application/json' };
  const contentType = request.headers.get('content-type');
  if (contentType !== null) headers['Content-Type'] = contentType;
  const authorization = request.headers.get('authorization');
  if (namespace === 'agent' && authorization !== null) headers.Authorization = authorization;
  const session = sessionCookie(request.headers.get('cookie'));
  if (namespace === 'account' && session !== null) headers.Cookie = `${SESSION_COOKIE_NAME}=${session}`;

  let body: string | undefined;
  if (request.method !== 'GET' && request.method !== 'HEAD' && request.method !== 'DELETE') {
    const incoming = await readBoundedText(request, CONTROL_BODY_LIMIT);
    if (!incoming.ok) return Response.json({ code: 'invalid_body', error: incoming.error }, { status: incoming.status });
    body = incoming.text;
  }

  let upstream: Response;
  try {
    upstream = await fetch(new URL(`/api/${namespace}/${path}`, base), {
      method: request.method,
      headers,
      body,
      cache: 'no-store',
      redirect: 'manual',
    });
  } catch (error: unknown) {
    console.error(`control plane unreachable: ${request.method} /api/${namespace}/${path}`, error);
    return Response.json(
      { code: 'service_unavailable', error: 'El servicio de cuentas no está disponible.' },
      { status: 503 },
    );
  }

  if (upstream.status >= 300 && upstream.status < 400) {
    return Response.json({ code: 'upstream_redirect', error: 'Respuesta no válida del servicio de cuentas.' }, { status: 502 });
  }
  const responseHeaders = new Headers({
    'Cache-Control': 'no-store',
    'Content-Type': upstream.headers.get('content-type') ?? 'application/json',
  });
  const setCookie = upstream.headers.get('set-cookie');
  if (setCookie !== null) responseHeaders.set('Set-Cookie', setCookie);
  return new Response(upstream.body, { status: upstream.status, headers: responseHeaders });
}

function sessionCookie(cookieHeader: string | null): string | null {
  if (cookieHeader === null) return null;
  const values = cookieHeader
    .split(';')
    .map((entry) => entry.trim())
    .filter((entry) => entry.startsWith(`${SESSION_COOKIE_NAME}=`))
    .map((entry) => entry.slice(SESSION_COOKIE_NAME.length + 1));
  if (values.length !== 1 || !/^[a-f0-9]{64}$/.test(values[0] ?? '')) return null;
  return values[0] ?? null;
}
