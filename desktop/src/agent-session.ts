import * as fs from 'node:fs';

export interface AgentSessionOptions {
  sessionFile: string;
  enabled: boolean;
  orchestratorUrl: string;
  token: string;
}

/** The user's choice to let an assistant drive Studio; off unless the file says otherwise. */
export function readAgentAccess(accessFile: string): boolean {
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(accessFile, 'utf8'));
    return typeof parsed === 'object' && parsed !== null && 'enabled' in parsed && parsed.enabled === true;
  } catch {
    return false;
  }
}

/**
 * Publishes the running orchestrator to zv-mcp. The token is written only when
 * access is on, so a closed switch leaves just the "switch off" marker.
 */
export function writeAgentSession({ sessionFile, enabled, orchestratorUrl, token }: AgentSessionOptions): void {
  const document = enabled ? { version: 1, enabled, orchestratorUrl, token } : { version: 1, enabled };
  const temp = `${sessionFile}.tmp`;
  fs.writeFileSync(temp, `${JSON.stringify(document)}\n`, { mode: 0o600 });
  fs.renameSync(temp, sessionFile);
}

export function removeAgentSession(sessionFile: string): void {
  fs.rmSync(sessionFile, { force: true });
}
