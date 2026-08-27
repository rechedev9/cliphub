export const AGENT_TRANSPORT_STORAGE_KEY = 'cliphub_agent_transport_v1' as const;

const AGENT_FRAGMENT_PATTERN = /^#agent=(\d{1,5})\.([a-f0-9]{64})$/;

export interface AgentTransportConfig {
  readonly version: 1;
  readonly origin: string;
  readonly capability: string;
}

export function parseAgentFragment(fragment: string): AgentTransportConfig | null {
  const match = AGENT_FRAGMENT_PATTERN.exec(fragment);
  if (match === null) return null;
  const port = Number(match[1]);
  const capability = match[2];
  if (!Number.isInteger(port) || port < 1 || port > 65_535 || capability === undefined) return null;
  return { version: 1, origin: `http://127.0.0.1:${port}`, capability };
}

export function parseStoredAgentConfig(raw: string | null): AgentTransportConfig | null {
  if (raw === null) return null;
  let value: unknown;
  try {
    value = JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null) return null;
  const candidate = value as Record<string, unknown>;
  if (candidate.version !== 1 || typeof candidate.origin !== 'string' || typeof candidate.capability !== 'string') {
    return null;
  }
  const fragment = `#agent=${new URL(candidate.origin).port}.${candidate.capability}`;
  const parsed = parseAgentFragment(fragment);
  return parsed?.origin === candidate.origin ? parsed : null;
}
