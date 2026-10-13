function originOf(url: string | undefined): string | null {
  if (!url) return null;
  try {
    return new URL(url).origin;
  } catch {
    return null;
  }
}

// The public origin: AUTH_URL when set (always, behind the proxy), else the request's own.
export function portalOrigin(
  request: Request,
  env: Record<string, string | undefined> = process.env,
): string | null {
  return originOf(env.AUTH_URL) ?? originOf(request.url);
}

// A cookie-authenticated mutation must come from the portal's own pages.
export function isSameOrigin(
  request: Request,
  env: Record<string, string | undefined> = process.env,
): boolean {
  const origin = request.headers.get("origin");
  const expected = portalOrigin(request, env);
  return origin !== null && expected !== null && origin === expected;
}
