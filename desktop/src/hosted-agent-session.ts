import { setTimeout as delay } from 'node:timers/promises';
import { ControlPlaneClient, type AgentRegistration } from './control-plane-client.ts';

const PAIRING_POLL_INTERVAL_MS = 3_000;
const HEARTBEAT_INTERVAL_MS = 30_000;
const CLOUD_RETRY_INTERVAL_MS = 15_000;

export interface HostedAgentSessionOptions {
  readonly webOrigin: string;
  readonly localWebPort: number;
  readonly browserCapability: string;
  readonly registration: AgentRegistration;
  readonly client: ControlPlaneClient;
  readonly openExternal: (url: string) => Promise<void>;
  readonly log: (line: string) => void;
}

export class HostedAgentSession {
  private readonly options: HostedAgentSessionOptions;
  private readonly controller = new AbortController();
  private background: Promise<void> | null = null;

  constructor(options: HostedAgentSessionOptions) {
    this.options = options;
  }

  async start(): Promise<void> {
    try {
      const pairing = await this.options.client.register(this.options.registration, this.controller.signal);
      await this.openPairing(pairing);
      this.background = this.supervise(pairing);
    } catch (error: unknown) {
      this.options.log(`[agent] cloud unavailable; local services remain active: ${String(error)}\n`);
      await this.options.openExternal(this.hostedURL());
      this.background = this.retryConnection();
    }
  }

  private async supervise(pairing: Awaited<ReturnType<ControlPlaneClient['register']>>): Promise<void> {
    try {
      if (pairing.status === 'pending') await this.waitForClaim();
      else await this.heartbeatLoop();
    } catch (error: unknown) {
      if (this.controller.signal.aborted) return;
      this.options.log(`[agent] cloud session interrupted: ${String(error)}\n`);
      await this.retryConnection();
    }
  }

  private async retryConnection(): Promise<void> {
    while (!this.controller.signal.aborted) {
      await delay(CLOUD_RETRY_INTERVAL_MS, undefined, { signal: this.controller.signal });
      try {
        const pairing = await this.options.client.register(this.options.registration, this.controller.signal);
        await this.openPairing(pairing);
        await this.supervise(pairing);
        return;
      } catch (error: unknown) {
        if (!this.controller.signal.aborted) this.options.log(`[agent] cloud retry failed: ${String(error)}\n`);
      }
    }
  }

  private openPairing(pairing: Awaited<ReturnType<ControlPlaneClient['register']>>): Promise<void> {
    return this.options.openExternal(this.hostedURL(pairing.status === 'pending' ? pairing.code : undefined));
  }

  private hostedURL(pairingCode?: string): string {
    return buildHostedStudioURL({
      webOrigin: this.options.webOrigin,
      localWebPort: this.options.localWebPort,
      browserCapability: this.options.browserCapability,
      pairingCode,
    });
  }

  stop(): void {
    this.controller.abort();
  }

  private async waitForClaim(): Promise<void> {
    while (!this.controller.signal.aborted) {
      await delay(PAIRING_POLL_INTERVAL_MS, undefined, { signal: this.controller.signal });
      if (await this.options.client.pairingStatus(this.options.registration.identity, this.controller.signal)) {
        await this.heartbeatLoop();
        return;
      }
    }
  }

  private async heartbeatLoop(): Promise<void> {
    while (!this.controller.signal.aborted) {
      await this.options.client.heartbeat(this.options.registration, this.controller.signal);
      await delay(HEARTBEAT_INTERVAL_MS, undefined, { signal: this.controller.signal });
    }
  }
}

export function buildHostedStudioURL(input: {
  readonly webOrigin: string;
  readonly localWebPort: number;
  readonly browserCapability: string;
  readonly pairingCode?: string;
}): string {
  const url = new URL('/connect', input.webOrigin);
  if (url.protocol !== 'https:' || url.pathname !== '/connect') throw new Error('hosted Studio origin must use HTTPS');
  if (!Number.isInteger(input.localWebPort) || input.localWebPort < 1 || input.localWebPort > 65_535) {
    throw new Error('local web port is invalid');
  }
  if (!/^[a-f0-9]{64}$/.test(input.browserCapability)) throw new Error('browser capability is invalid');
  if (input.pairingCode !== undefined) {
    if (!/^[A-Z2-7]{10}$/.test(input.pairingCode)) throw new Error('pairing code is invalid');
    url.searchParams.set('pair', input.pairingCode);
  }
  url.hash = `agent=${input.localWebPort}.${input.browserCapability}`;
  return url.toString();
}
