const ALLOWED_FETCH_SITES = new Set(['none', 'same-origin']);

const INVALID_HOST_ERROR = 'local API host rejected';
const CROSS_SITE_ERROR = 'cross-site request blocked';
export const PROXY_MUTATION_CAPABILITY_ENV = 'CLIPHUB_PROXY_MUTATION_CAPABILITY' as const;
export const PROXY_MUTATION_CAPABILITY_COOKIE = 'cliphub_proxy_capability' as const;
export const MUTATION_CAPABILITY_ERROR = 'local API mutation capability required' as const;
export const PROXY_BOOTSTRAP_CAPABILITY_ENV = 'CLIPHUB_PROXY_BOOTSTRAP_CAPABILITY' as const;
export const BOOTSTRAP_CAPABILITY_ERROR = 'local API bootstrap capability required' as const;
export const HOSTED_ORIGIN_ENV = 'CLIPHUB_HOSTED_ORIGIN' as const;
export const HOSTED_BROWSER_CAPABILITY_ENV = 'CLIPHUB_HOSTED_BROWSER_CAPABILITY' as const;
export const HOSTED_BROWSER_CAPABILITY_ERROR = 'hosted browser capability required' as const;

const MUTATION_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

/**
 * Validates that an API request targets the loopback Studio origin and was not
 * initiated by another browser origin. The Host check also blocks DNS
 * rebinding: an attacker-controlled hostname is rejected even if DNS resolves
 * it to 127.0.0.1.
 *
 * Reads remain available to origin-less local clients with an explicit
 * loopback Host and port. Mutations additionally require the HttpOnly
 * per-launch capability cookie seeded by the desktop main process; the browser
 * never receives the orchestrator token or the capability value in JavaScript.
 */
export async function localAPIRequestError(headers: Headers, method = 'GET'): Promise<string | undefined> {
  const localOrigin = localAPIOrigin(headers);
  if (localOrigin === undefined) return INVALID_HOST_ERROR;
  const host = new URL(localOrigin).host;

  const origin = headers.get('origin');
  if (origin !== null && origin === configuredHostedOrigin()) {
    const supplied = bearerCapability(headers.get('authorization'));
    const expected = process.env[HOSTED_BROWSER_CAPABILITY_ENV];
    if (!expected || !supplied || !(await capabilityMatches(supplied, expected))) {
      return HOSTED_BROWSER_CAPABILITY_ERROR;
    }
    return undefined;
  }

  const fetchSite = headers.get('sec-fetch-site');
  if (fetchSite !== null && !ALLOWED_FETCH_SITES.has(fetchSite.toLowerCase())) {
    return CROSS_SITE_ERROR;
  }

  if (origin !== null) {
    try {
      const parsed = new URL(origin);
      if (
        parsed.username !== ''
        || parsed.password !== ''
        || parsed.pathname !== '/'
        || parsed.search !== ''
        || parsed.hash !== ''
        || parsed.host.toLowerCase() !== host.toLowerCase()
      ) {
        return CROSS_SITE_ERROR;
      }
    } catch {
      return CROSS_SITE_ERROR;
    }
  }

  if (!MUTATION_METHODS.has(method.toUpperCase())) return undefined;

  const expected = process.env[PROXY_MUTATION_CAPABILITY_ENV];
  const supplied = cookieValue(headers, PROXY_MUTATION_CAPABILITY_COOKIE);
  if (!expected || !supplied || !(await capabilityMatches(supplied, expected))) {
    return MUTATION_CAPABILITY_ERROR;
  }
  return undefined;
}

export function localAPICORSHeaders(headers: Headers): Headers | null {
  const origin = headers.get('origin');
  if (origin === null || origin !== configuredHostedOrigin()) return null;
  return new Headers({
    'Access-Control-Allow-Origin': origin,
    'Access-Control-Allow-Methods': 'GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS',
    'Access-Control-Allow-Headers': 'Authorization, Content-Type, Range',
    'Access-Control-Allow-Private-Network': 'true',
    'Access-Control-Expose-Headers': 'Accept-Ranges, Content-Length, Content-Range',
    'Access-Control-Max-Age': '600',
    Vary: 'Origin',
  });
}

export function withLocalCORS(request: Request, response: Response): Response {
  const cors = localAPICORSHeaders(request.headers);
  if (cors === null) return response;
  const headers = new Headers(response.headers);
  cors.forEach((value, name) => headers.set(name, value));
  return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
}

/**
 * Returns the exact validated loopback origin used by the browser request.
 * Next's development Request.url can be canonicalized to `localhost` even
 * when the Host header and capability cookie belong to `127.0.0.1`. Redirects
 * must preserve this origin or the host-only HttpOnly cookie is immediately
 * lost.
 */
export function localAPIOrigin(headers: Headers): string | undefined {
  const host = headers.get('host')?.trim() ?? '';
  return isLoopbackHostWithPort(host) ? `http://${host}` : undefined;
}

/**
 * Validates the one standalone bootstrap route. It deliberately uses the read
 * guard first so this route cannot become an alternate cross-origin mutation
 * path, then requires a separate server-only bootstrap capability.
 */
export async function localAPIBootstrapError(headers: Headers, supplied: string | null): Promise<string | undefined> {
  const localError = await localAPIRequestError(headers, 'GET');
  if (localError !== undefined) return localError;

  const expected = process.env[PROXY_BOOTSTRAP_CAPABILITY_ENV];
  if (!expected || !supplied || !(await capabilityMatches(supplied, expected))) {
    return BOOTSTRAP_CAPABILITY_ERROR;
  }
  return undefined;
}

/**
 * Hashing both values first makes the byte comparison fixed-width. This works
 * in Next middleware's Web Crypto runtime as well as the Node route runtime.
 */
async function capabilityMatches(supplied: string, expected: string): Promise<boolean> {
  const bytes = new TextEncoder();
  const [suppliedDigest, expectedDigest] = await Promise.all([
    crypto.subtle.digest('SHA-256', bytes.encode(supplied)),
    crypto.subtle.digest('SHA-256', bytes.encode(expected)),
  ]);
  const left = new Uint8Array(suppliedDigest);
  const right = new Uint8Array(expectedDigest);
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= (left[index] ?? 0) ^ (right[index] ?? 0);
  }
  return difference === 0;
}

function configuredHostedOrigin(): string | null {
  const raw = process.env[HOSTED_ORIGIN_ENV];
  if (!raw) return null;
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'https:' || parsed.pathname !== '/' || parsed.search !== '' || parsed.hash !== '') return null;
    return parsed.origin;
  } catch {
    return null;
  }
}

function bearerCapability(value: string | null): string | null {
  if (value === null || !value.startsWith('Bearer ')) return null;
  const token = value.slice('Bearer '.length);
  return /^[a-f0-9]{64}$/.test(token) ? token : null;
}

function cookieValue(headers: Headers, name: string): string | undefined {
  const cookie = headers.get('cookie');
  if (!cookie) return undefined;

  let value: string | undefined;
  for (const entry of cookie.split(';')) {
    const trimmed = entry.trim();
    const separator = trimmed.indexOf('=');
    if (separator < 1 || trimmed.slice(0, separator) !== name) continue;
    // Duplicate cookie names are ambiguous. Reject rather than depending on
    // parsing order across browser and proxy implementations.
    if (value !== undefined) return undefined;
    value = trimmed.slice(separator + 1);
  }
  return value;
}

function isLoopbackHostWithPort(host: string): boolean {
  const match = /^(localhost|\[::1\]|(\d{1,3}(?:\.\d{1,3}){3})):(\d{1,5})$/i.exec(host);
  if (match === null) return false;

  const port = Number(match[3]);
  if (!Number.isInteger(port) || port < 1 || port > 65_535) return false;

  const hostname = match[1]?.toLowerCase();
  if (hostname === 'localhost' || hostname === '[::1]') return true;

  const octets = match[2]?.split('.').map(Number);
  return octets !== undefined
    && octets.length === 4
    && octets[0] === 127
    && octets.every((octet) => Number.isInteger(octet) && octet >= 0 && octet <= 255);
}
