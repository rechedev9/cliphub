// The public demo: a visitor with no account reads one workspace, chosen by the
// server's CLIPMIND_DEMO_WORKSPACE, and can change nothing in it.

/** Set by /demo; readable by page code so the banner can show. Not a credential. */
export const DEMO_COOKIE = 'cm_demo';
/** Set by the proxy on a demo visitor's request; a value from the browser is discarded. */
export const DEMO_HEADER = 'x-cm-demo';
/** What the ClipMind server reads to serve the demo workspace. */
export const DEMO_UPSTREAM_HEADER = 'X-ClipHub-Demo';

export const DEMO_PATH = '/demo';
const DEMO_COOKIE_MAX_AGE_SECONDS = 7 * 24 * 60 * 60;

/** Pages that act on an account; a demo visitor is sent to sign up instead. */
const ACCOUNT_PAGES: readonly string[] = ['/cuenta', '/equipo', '/importar', '/invitacion'];

export type DemoDecision = 'forward' | 'read_only' | 'sign_up';

/** Where /demo lands, from CLIPHUB_DEMO_HOME; undefined turns the demo off. */
export function demoHome(): string | undefined {
  const home = process.env.CLIPHUB_DEMO_HOME ?? '';
  return home.startsWith('/') && !home.startsWith('//') && !home.includes('\\') ? home : undefined;
}

function isUnder(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

/** What the proxy does with a demo visitor's request. */
export function demoDecision(method: string, pathname: string): DemoDecision {
  if (ACCOUNT_PAGES.some((page) => isUnder(pathname, page))) return 'sign_up';
  const isApi = isUnder(pathname, '/api');
  if (isApi && isUnder(pathname, '/api/cm')) return 'sign_up';
  if (isApi && method !== 'GET' && method !== 'HEAD') return 'read_only';
  return 'forward';
}

export interface DemoCookieOptions {
  path: string;
  maxAge: number;
  sameSite: 'lax';
  secure: boolean;
  httpOnly: false;
}

export function demoCookieOptions(secure: boolean): DemoCookieOptions {
  return { path: '/', maxAge: DEMO_COOKIE_MAX_AGE_SECONDS, sameSite: 'lax', secure, httpOnly: false };
}

/** True when the browser carries the demo cookie (page code reads document.cookie). */
export function hasDemoCookie(cookieHeader: string): boolean {
  return cookieHeader.split(';').some((part) => part.trim() === `${DEMO_COOKIE}=1`);
}
