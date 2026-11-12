// The web edition keeps the sign-in in two HttpOnly cookies. The proxy refreshes
// the short access token from the long refresh token and hands the fresh one to
// route handlers in a request header, so the browser never holds a token in JS.

export const ACCESS_COOKIE = 'cm_access';
export const REFRESH_COOKIE = 'cm_refresh';
/** Set by the proxy on every web-edition request; a value from the browser is discarded. */
export const ACCESS_HEADER = 'x-cm-access';

const REFRESH_MAX_AGE_SECONDS = 60 * 24 * 60 * 60;
/** Refresh a little early so a request never reaches the server with a token about to lapse. */
const EXPIRY_MARGIN_MS = 30_000;
/** One refresh answers every request that arrives with the same old cookie, for this long. */
const REFRESH_SHARE_MS = 60_000;

export function clipMindUrl(): string {
  return (process.env.CLIPMIND_URL ?? 'http://127.0.0.1:8090').replace(/\/$/, '');
}

export interface TokenSet {
  access: string;
  refresh: string;
  expiresAt: number;
  workspaceId: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** Reads the server's login or refresh answer; anything else is not a token set. */
export function parseTokenSet(value: unknown): TokenSet | undefined {
  if (!isRecord(value)) return undefined;
  const { access_token: access, refresh_token: refresh, expires_at: expiresAt, workspace_id: workspaceId } = value;
  if (typeof access !== 'string' || typeof refresh !== 'string' || typeof expiresAt !== 'number') return undefined;
  return { access, refresh, expiresAt, workspaceId: typeof workspaceId === 'string' ? workspaceId : '' };
}

/** The expiry the access token carries, in milliseconds; 0 when it cannot be read. */
export function accessExpiry(token: string): number {
  const body = token.split('.')[0] ?? '';
  try {
    const json = atob(body.replace(/-/g, '+').replace(/_/g, '/'));
    const claims: unknown = JSON.parse(json);
    return isRecord(claims) && typeof claims.exp === 'number' ? claims.exp : 0;
  } catch {
    return 0;
  }
}

interface SharedRefresh {
  at: number;
  result: Promise<TokenSet | undefined>;
}

const shared = new Map<string, SharedRefresh>();

/**
 * Trades a refresh token for a new pair. The server rotates the token and treats a
 * second use of the old one as theft, so parallel requests of one page must share
 * a single exchange instead of each making their own.
 */
export function refreshTokens(refresh: string, workspaceId?: string): Promise<TokenSet | undefined> {
  const now = Date.now();
  for (const [key, entry] of shared) {
    if (now - entry.at > REFRESH_SHARE_MS) shared.delete(key);
  }
  const key = `${refresh}|${workspaceId ?? ''}`;
  const existing = shared.get(key);
  if (existing !== undefined) return existing.result;
  const result = exchange(refresh, workspaceId);
  shared.set(key, { at: now, result });
  return result;
}

async function exchange(refresh: string, workspaceId?: string): Promise<TokenSet | undefined> {
  try {
    const response = await fetch(`${clipMindUrl()}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refresh, workspace_id: workspaceId ?? '' }),
      cache: 'no-store',
    });
    if (!response.ok) return undefined;
    return parseTokenSet(await response.json());
  } catch (error) {
    console.error('clipmind refresh failed', error);
    return undefined;
  }
}

interface CookieReader {
  get(name: string): { value: string } | undefined;
}

export interface Session {
  access: string;
  /** Present when this request had to refresh; the response must set the new cookies. */
  renewed?: TokenSet;
}

/** The request's live access token, refreshing it when it has (nearly) expired. */
export async function resolveSession(cookies: CookieReader): Promise<Session | undefined> {
  const access = cookies.get(ACCESS_COOKIE)?.value;
  if (access !== undefined && accessExpiry(access) - Date.now() > EXPIRY_MARGIN_MS) return { access };
  const refresh = cookies.get(REFRESH_COOKIE)?.value;
  if (refresh === undefined) return undefined;
  const renewed = await refreshTokens(refresh);
  return renewed === undefined ? undefined : { access: renewed.access, renewed };
}

export interface CookieSpec {
  name: string;
  value: string;
  options: { httpOnly: true; sameSite: 'lax'; secure: boolean; path: '/'; maxAge: number };
}

export function sessionCookies(tokens: TokenSet, secure: boolean): CookieSpec[] {
  const accessSeconds = Math.max(1, Math.floor((tokens.expiresAt - Date.now()) / 1000));
  return [
    { name: ACCESS_COOKIE, value: tokens.access, options: { httpOnly: true, sameSite: 'lax', secure, path: '/', maxAge: accessSeconds } },
    { name: REFRESH_COOKIE, value: tokens.refresh, options: { httpOnly: true, sameSite: 'lax', secure, path: '/', maxAge: REFRESH_MAX_AGE_SECONDS } },
  ];
}

export function clearedCookies(secure: boolean): CookieSpec[] {
  return [ACCESS_COOKIE, REFRESH_COOKIE].map((name) => ({
    name,
    value: '',
    options: { httpOnly: true, sameSite: 'lax', secure, path: '/', maxAge: 0 },
  }));
}
