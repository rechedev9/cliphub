export const WEB_MODE_ENV = 'CLIPHUB_WEB_MODE' as const;
export const HOSTED_WEB_MODE = 'hosted' as const;
export const ACCOUNT_API_PREFIX = '/api/account/' as const;
export const AGENT_API_PREFIX = '/api/agent/' as const;

type RuntimeEnvironment = Readonly<Record<string, string | undefined>>;

export function isHostedWebMode(environment: RuntimeEnvironment = process.env): boolean {
  return environment[WEB_MODE_ENV] === HOSTED_WEB_MODE;
}

export function configuredControlPlaneURL(environment: RuntimeEnvironment = process.env): URL | null {
  const raw = environment.CLIPHUB_CONTROL_PLANE_URL;
  if (!raw) return null;
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
    parsed.pathname = '/';
    parsed.search = '';
    parsed.hash = '';
    return parsed;
  } catch {
    return null;
  }
}

export function isHostedPublicPath(pathname: string): boolean {
  return pathname === '/login'
    || pathname === '/register'
    || pathname === '/connect'
    || pathname === '/api/installer'
    || pathname === '/generated/agent-sw.js'
    || pathname.startsWith(ACCOUNT_API_PREFIX)
    || pathname.startsWith(AGENT_API_PREFIX);
}
