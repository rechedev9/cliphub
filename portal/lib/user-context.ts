import { auth } from "../auth.ts";
import { isAdminUser } from "./admin.ts";
import { authenticateDevice } from "./device-auth.ts";
import { jsonError, unauthorized } from "./http.ts";
import { isSameOrigin } from "./same-origin.ts";
import { bearerToken } from "./tokens.ts";

export interface UserContext {
  userId: string;
  deviceId: string | null;
  via: "device" | "session";
}

export type Resolved<T> = { ok: true; user: T } | { ok: false; response: Response };

function isRead(request: Request): boolean {
  return request.method === "GET" || request.method === "HEAD";
}

function forbidden(): Response {
  return jsonError(403, "forbidden");
}

async function sessionUserId(request: Request): Promise<Resolved<string>> {
  const session = await auth();
  const userId = session?.user?.id;
  if (!userId) return { ok: false, response: unauthorized() };
  // A cookie rides along on cross-site requests, so a mutation must prove where it came from.
  if (!isRead(request) && !isSameOrigin(request)) {
    return { ok: false, response: forbidden() };
  }
  return { ok: true, user: userId };
}

// Who is calling a user route: a linked Studio (Bearer chd_ token) or a signed-in browser.
// A request that carries a Bearer token is never authenticated by its cookies.
export async function resolveUser(request: Request): Promise<Resolved<UserContext>> {
  const token = bearerToken(request);
  if (token !== null) {
    const device = await authenticateDevice(token, Date.now());
    if (!device) return { ok: false, response: unauthorized() };
    return { ok: true, user: { ...device, via: "device" } };
  }
  const session = await sessionUserId(request);
  if (!session.ok) return session;
  return { ok: true, user: { userId: session.user, deviceId: null, via: "session" } };
}

// For routes that only a browser session may call (managing devices, approving a link).
export async function resolveSessionUser(request: Request): Promise<Resolved<UserContext>> {
  if (bearerToken(request) !== null) return { ok: false, response: forbidden() };
  return resolveUser(request);
}

// Admin routes: a session whose account is on the ADMIN_ACCOUNTS allowlist.
export async function resolveAdmin(request: Request): Promise<Resolved<string>> {
  if (bearerToken(request) !== null) return { ok: false, response: forbidden() };
  const session = await sessionUserId(request);
  if (!session.ok) return session;
  if (!(await isAdminUser(session.user))) return { ok: false, response: forbidden() };
  return session;
}
