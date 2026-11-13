// Browser-side calls to the account API (`/api/cm`). Tokens stay in HttpOnly
// cookies; this module only sees JSON.

export interface CmUser {
  id: string;
  email: string;
  username: string;
  display_name: string;
}

export interface CmWorkspace {
  id: string;
  name: string;
  owner_id: string;
  role?: string;
}

export interface CmMember {
  user_id: string;
  email: string;
  username: string;
  display_name: string;
  role: string;
}

export interface CmInvitation {
  id: string;
  email: string;
  expires_at: number;
}

export interface CmMe {
  user: CmUser;
  workspace: CmWorkspace | null;
  workspaces: CmWorkspace[];
}

export interface CmSession {
  id: string;
  device: string;
  last_used_at: number;
  current: boolean;
}

export interface CmApiKey {
  id: string;
  name: string;
  workspace_id: string;
  last_used_at: number;
}

export type CmResult<T> = { ok: true; data: T } | { ok: false; status: number; code: string; message: string };

const MESSAGES: Readonly<Record<string, string>> = {
  invalid_credentials: 'El usuario o la contraseña no son correctos.',
  username_taken: 'Ese nombre de usuario ya está en uso. Elige otro.',
  email_not_confirmed: 'Confirma tu correo antes de entrar: te enviamos un enlace.',
  locked: 'Demasiados intentos fallidos. Espera unos minutos e inténtalo de nuevo.',
  token_invalid: 'El enlace no es válido o ha caducado.',
  forbidden: 'No tienes permiso para hacer esto.',
  not_found: 'No se ha encontrado.',
  conflict: 'Eso ya existe o choca con otro cambio.',
  unauthorized: 'Tu sesión ha terminado. Entra de nuevo.',
  unavailable: 'No se puede contactar con ClipMind. Inténtalo en un momento.',
  rate_limited: 'Demasiados intentos seguidos. Espera un momento.',
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function failure(status: number, body: unknown): CmResult<never> {
  const fallback = status === 429 ? 'rate_limited' : 'error';
  const code = isRecord(body) && typeof body.code === 'string' ? body.code : fallback;
  // A validation message comes from the server in English; the rest map to Spanish.
  const detail = code === 'invalid_input' && isRecord(body) && typeof body.error === 'string' ? body.error : '';
  const message = MESSAGES[code] ?? (detail !== '' ? `Datos no válidos: ${detail}` : 'Algo ha fallado. Inténtalo de nuevo.');
  return { ok: false, status, code, message };
}

/** One call to the account API; never throws. */
export async function cm<T>(path: string, init: { method?: string; body?: unknown; parse: (value: unknown) => T }): Promise<CmResult<T>> {
  try {
    const response = await fetch(`/api/cm/${path}`, {
      method: init.method ?? 'GET',
      headers: init.body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
      cache: 'no-store',
    });
    const body: unknown = await response.json().catch(() => undefined);
    if (!response.ok) return failure(response.status, body);
    return { ok: true, data: init.parse(body) };
  } catch {
    return failure(503, { code: 'unavailable' });
  }
}

export function isDone(value: unknown): true {
  void value;
  return true;
}

function list<T>(value: unknown, key: string, read: (item: unknown) => T | undefined): T[] {
  const items = isRecord(value) ? value[key] : value;
  if (!Array.isArray(items)) return [];
  return items.flatMap((item) => {
    const parsed = read(item);
    return parsed === undefined ? [] : [parsed];
  });
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function num(value: unknown): number {
  return typeof value === 'number' ? value : 0;
}

function readWorkspace(value: unknown): CmWorkspace | undefined {
  if (!isRecord(value)) return undefined;
  return { id: str(value.id), name: str(value.name), owner_id: str(value.owner_id), role: str(value.role) };
}

export function parseMe(value: unknown): CmMe {
  const body = isRecord(value) ? value : {};
  const user = isRecord(body.user) ? body.user : {};
  return {
    user: { id: str(user.id), email: str(user.email), username: str(user.username), display_name: str(user.display_name) },
    workspace: readWorkspace(body.workspace) ?? null,
    workspaces: list(body.workspaces, '', readWorkspace),
  };
}

export function parseWorkspaces(value: unknown): CmWorkspace[] {
  return list(value, 'workspaces', readWorkspace);
}

export function parseMembers(value: unknown): CmMember[] {
  return list(value, 'members', (item) =>
    isRecord(item)
      ? { user_id: str(item.user_id), email: str(item.email), username: str(item.username), display_name: str(item.display_name), role: str(item.role) }
      : undefined,
  );
}

export function parseInvitations(value: unknown): CmInvitation[] {
  return list(value, 'invitations', (item) => (isRecord(item) ? { id: str(item.id), email: str(item.email), expires_at: num(item.expires_at) } : undefined));
}

export function parseSessions(value: unknown): CmSession[] {
  return list(value, 'sessions', (item) =>
    isRecord(item) ? { id: str(item.id), device: str(item.device), last_used_at: num(item.last_used_at), current: item.current === true } : undefined,
  );
}

export function parseApiKeys(value: unknown): CmApiKey[] {
  return list(value, 'tokens', (item) =>
    isRecord(item) ? { id: str(item.id), name: str(item.name), workspace_id: str(item.workspace_id), last_used_at: num(item.last_used_at) } : undefined,
  );
}

export function parseSecret(value: unknown): string {
  return isRecord(value) ? str(value.secret) : '';
}

/** How a person reads in a list: the shown name, then the username and email that are set. */
export function accountLabel(person: Pick<CmUser, 'email' | 'username' | 'display_name'>): { name: string; detail: string } {
  const handles = [person.username, person.email].filter((part) => part !== '');
  const name = person.display_name === '' ? (handles[0] ?? '') : person.display_name;
  return { name, detail: handles.filter((part) => part !== name).join(' · ') };
}

/** Where to go after signing in: only a path inside this app, never another site. */
export function safeNext(value: string | null): string {
  if (value === null || !value.startsWith('/')) return '/partidas';
  // The URL parser drops tabs and reads backslashes as slashes, as the router does.
  const url = URL.parse(value, 'http://cliphub.invalid');
  return url?.origin === 'http://cliphub.invalid' ? `${url.pathname}${url.search}${url.hash}` : '/partidas';
}
