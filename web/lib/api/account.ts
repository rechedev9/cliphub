export interface AccountUser {
  readonly id: string;
  readonly email: string;
}

export interface AccountDevice {
  readonly id: string;
  readonly name: string;
  readonly platform: string;
  readonly version: string;
  readonly online: boolean;
  readonly lastSeen: string;
}

export type AccountResult<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly code: string; readonly error: string };

const ACCOUNT_API = '/api/account' as const;

export async function registerAccount(email: string, password: string): Promise<AccountResult<AccountUser>> {
  return authenticate('register', email, password);
}

export async function loginAccount(email: string, password: string): Promise<AccountResult<AccountUser>> {
  return authenticate('login', email, password);
}

export async function loadAccountSession(): Promise<AccountResult<AccountUser>> {
  const response = await fetch(`${ACCOUNT_API}/session`, { cache: 'no-store' });
  return userResult(response);
}

export async function logoutAccount(): Promise<AccountResult<undefined>> {
  const response = await fetch(`${ACCOUNT_API}/logout`, { method: 'POST' });
  if (response.ok) return { ok: true, value: undefined };
  return errorResult(response);
}

export async function listAccountDevices(): Promise<AccountResult<readonly AccountDevice[]>> {
  const response = await fetch(`${ACCOUNT_API}/devices`, { cache: 'no-store' });
  if (!response.ok) return errorResult(response);
  const body = await response.json() as unknown;
  if (typeof body !== 'object' || body === null || !Array.isArray((body as Record<string, unknown>).devices)) {
    return invalidResponse();
  }
  const devices: AccountDevice[] = [];
  for (const item of (body as { devices: unknown[] }).devices) {
    const device = parseDevice(item);
    if (device === null) return invalidResponse();
    devices.push(device);
  }
  return { ok: true, value: devices };
}

export async function claimAccountDevice(code: string): Promise<AccountResult<AccountDevice>> {
  const response = await fetch(`${ACCOUNT_API}/devices/claim`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (!response.ok) return errorResult(response);
  const body = await response.json() as unknown;
  if (typeof body !== 'object' || body === null) return invalidResponse();
  const device = parseDevice((body as Record<string, unknown>).device);
  return device === null ? invalidResponse() : { ok: true, value: device };
}

async function authenticate(action: 'register' | 'login', email: string, password: string): Promise<AccountResult<AccountUser>> {
  const response = await fetch(`${ACCOUNT_API}/${action}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  return userResult(response);
}

async function userResult(response: Response): Promise<AccountResult<AccountUser>> {
  if (!response.ok) return errorResult(response);
  const body = await response.json() as unknown;
  if (typeof body !== 'object' || body === null) return invalidResponse();
  const user = (body as Record<string, unknown>).user;
  if (typeof user !== 'object' || user === null) return invalidResponse();
  const candidate = user as Record<string, unknown>;
  if (typeof candidate.id !== 'string' || typeof candidate.email !== 'string') return invalidResponse();
  return { ok: true, value: { id: candidate.id, email: candidate.email } };
}

async function errorResult(response: Response): Promise<AccountResult<never>> {
  let body: unknown;
  try {
    body = await response.json() as unknown;
  } catch {
    return { ok: false, code: 'request_failed', error: `La solicitud falló (${response.status}).` };
  }
  if (typeof body !== 'object' || body === null) return invalidResponse();
  const candidate = body as Record<string, unknown>;
  return {
    ok: false,
    code: typeof candidate.code === 'string' ? candidate.code : 'request_failed',
    error: typeof candidate.error === 'string' ? candidate.error : `La solicitud falló (${response.status}).`,
  };
}

function parseDevice(value: unknown): AccountDevice | null {
  if (typeof value !== 'object' || value === null) return null;
  const candidate = value as Record<string, unknown>;
  if (
    typeof candidate.id !== 'string'
    || typeof candidate.name !== 'string'
    || typeof candidate.platform !== 'string'
    || typeof candidate.version !== 'string'
  ) return null;
  return {
    id: candidate.id,
    name: candidate.name,
    platform: candidate.platform,
    version: candidate.version,
    online: candidate.online === true,
    lastSeen: typeof candidate.lastSeen === 'string' ? candidate.lastSeen : '',
  };
}

function invalidResponse(): AccountResult<never> {
  return { ok: false, code: 'invalid_response', error: 'El servidor devolvió una respuesta no válida.' };
}
