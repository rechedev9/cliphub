import type { DeviceIdentity } from './device-identity.ts';

const PAIRING_CODE_PATTERN = /^[A-Z2-7]{10}$/;

export interface AgentRegistration {
  readonly identity: DeviceIdentity;
  readonly name: string;
  readonly platform: string;
  readonly version: string;
}

export type PairingState =
  | { readonly status: 'pending'; readonly code: string; readonly expiresAt: string }
  | { readonly status: 'claimed' };

export class ControlPlaneClient {
  private readonly baseURL: URL;
  private readonly request: typeof fetch;

  constructor(baseURL: string, request: typeof fetch = fetch) {
    const parsed = new URL(baseURL);
    if (parsed.protocol !== 'https:' && !(parsed.protocol === 'http:' && isLoopback(parsed.hostname))) {
      throw new Error('control plane URL must use HTTPS or HTTP loopback');
    }
    parsed.pathname = '/';
    parsed.search = '';
    parsed.hash = '';
    this.baseURL = parsed;
    this.request = request;
  }

  async register(input: AgentRegistration, signal?: AbortSignal): Promise<PairingState> {
    const response = await this.request(this.url('/api/agent/pairings'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        deviceId: input.identity.deviceId,
        name: input.name,
        platform: input.platform,
        version: input.version,
        secret: input.identity.secret,
      }),
      signal,
    });
    if (response.status === 400) {
      const claimed = await this.pairingStatus(input.identity, signal);
      if (claimed) return { status: 'claimed' };
    }
    if (!response.ok) throw await controlPlaneError(response, 'register device');
    const body = await response.json() as unknown;
    const pairing = pairingFromResponse(body);
    return { status: 'pending', code: pairing.code, expiresAt: pairing.expiresAt };
  }

  async pairingStatus(identity: DeviceIdentity, signal?: AbortSignal): Promise<boolean> {
    const response = await this.request(this.url(`/api/agent/pairings/${identity.deviceId}/status`), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${identity.secret}`,
        'Content-Type': 'application/json',
      },
      body: '{}',
      signal,
    });
    if (!response.ok) throw await controlPlaneError(response, 'read pairing status');
    const body = await response.json() as unknown;
    if (typeof body !== 'object' || body === null || typeof (body as Record<string, unknown>).claimed !== 'boolean') {
      throw new Error('control plane returned an invalid pairing status');
    }
    return (body as { claimed: boolean }).claimed;
  }

  async heartbeat(input: AgentRegistration, signal?: AbortSignal): Promise<void> {
    const response = await this.request(this.url('/api/agent/heartbeat'), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${input.identity.secret}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ deviceId: input.identity.deviceId, version: input.version }),
      signal,
    });
    if (!response.ok) throw await controlPlaneError(response, 'send device heartbeat');
  }

  private url(pathname: string): string {
    return new URL(pathname, this.baseURL).toString();
  }
}

function pairingFromResponse(value: unknown): { code: string; expiresAt: string } {
  if (typeof value !== 'object' || value === null) throw new Error('control plane returned an invalid pairing');
  const pairing = (value as Record<string, unknown>).pairing;
  if (typeof pairing !== 'object' || pairing === null) throw new Error('control plane returned an invalid pairing');
  const candidate = pairing as Record<string, unknown>;
  if (
    typeof candidate.code !== 'string'
    || !PAIRING_CODE_PATTERN.test(candidate.code)
    || typeof candidate.expiresAt !== 'string'
    || Number.isNaN(Date.parse(candidate.expiresAt))
  ) {
    throw new Error('control plane returned an invalid pairing');
  }
  return { code: candidate.code, expiresAt: candidate.expiresAt };
}

async function controlPlaneError(response: Response, operation: string): Promise<Error> {
  let code = `HTTP ${response.status}`;
  try {
    const body = await response.json() as unknown;
    if (typeof body === 'object' && body !== null && typeof (body as Record<string, unknown>).code === 'string') {
      code = (body as { code: string }).code;
    }
  } catch {
    // A non-JSON gateway failure is still represented by its HTTP status.
  }
  return new Error(`${operation}: ${code}`);
}

function isLoopback(hostname: string): boolean {
  return hostname === '127.0.0.1' || hostname === 'localhost' || hostname === '[::1]';
}
