import * as fs from 'node:fs';

/** The running Studio services zv-mcp needs to reach, minted per boot. */
export interface AgentSessionTarget {
  orchestratorUrl: string;
  token: string;
  webOrigin: string;
}

/** File operations the session store uses; tests replace them to simulate Windows lock failures. */
export interface AgentSessionFs {
  writeFileSync(file: string, data: string, options: { mode: number }): void;
  renameSync(from: string, to: string): void;
  rmSync(file: string, options: { force: true }): void;
  sleepSync(ms: number): void;
}

export interface AgentSessionOptions extends AgentSessionTarget {
  sessionFile: string;
  enabled: boolean;
  fsOps?: AgentSessionFs;
}

export interface SetAgentAccessOptions {
  accessFile: string;
  sessionFile: string;
  enabled: boolean;
  /** The services of the current boot, or null while none is up. */
  running: AgentSessionTarget | null;
  fsOps?: AgentSessionFs;
}

const RENAME_ATTEMPTS = 5;
const RENAME_RETRY_DELAY_MS = 50;
const RETRYABLE_RENAME_CODES = new Set(['EPERM', 'EBUSY', 'EACCES']);

const nodeFs: AgentSessionFs = {
  writeFileSync: (file, data, options) => fs.writeFileSync(file, data, options),
  renameSync: (from, to) => fs.renameSync(from, to),
  rmSync: (file, options) => fs.rmSync(file, options),
  sleepSync: (ms) => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms),
};

/** The user's choice to let an assistant drive Studio; off unless the file says otherwise. */
export function readAgentAccess(accessFile: string): boolean {
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(accessFile, 'utf8'));
    return typeof parsed === 'object' && parsed !== null && 'enabled' in parsed && parsed.enabled === true;
  } catch {
    return false;
  }
}

/** Publishes the running orchestrator to zv-mcp; the token is written only while access is on. */
export function writeAgentSession({
  sessionFile,
  enabled,
  orchestratorUrl,
  token,
  webOrigin,
  fsOps = nodeFs,
}: AgentSessionOptions): void {
  if (enabled) {
    writeFileAtomic({ file: sessionFile, document: { version: 1, enabled, orchestratorUrl, token, webOrigin }, fsOps });
    return;
  }
  try {
    writeFileAtomic({ file: sessionFile, document: { version: 1, enabled }, fsOps });
  } catch {
    removeAgentSession(sessionFile, fsOps);
  }
}

/** Stores the switch and republishes a running boot; the session goes first so no token outlives a failure. */
export function setAgentAccess({ accessFile, sessionFile, enabled, running, fsOps = nodeFs }: SetAgentAccessOptions): void {
  if (running !== null) writeAgentSession({ sessionFile, enabled, ...running, fsOps });
  try {
    writeFileAtomic({ file: accessFile, document: { enabled }, fsOps });
  } catch (error) {
    // The choice was not stored, so a token published for it must not stay.
    if (enabled && running !== null) {
      try {
        removeAgentSession(sessionFile, fsOps);
      } catch {
        // The original failure is the one worth reporting.
      }
    }
    throw error;
  }
}

export function removeAgentSession(sessionFile: string, fsOps: AgentSessionFs = nodeFs): void {
  fsOps.rmSync(sessionFile, { force: true });
}

interface WriteFileAtomicOptions {
  file: string;
  document: object;
  fsOps: AgentSessionFs;
}

function writeFileAtomic({ file, document, fsOps }: WriteFileAtomicOptions): void {
  const temp = `${file}.tmp`;
  try {
    fsOps.writeFileSync(temp, `${JSON.stringify(document)}\n`, { mode: 0o600 });
    renameWithRetry({ from: temp, to: file, fsOps });
  } catch (error) {
    try {
      fsOps.rmSync(temp, { force: true });
    } catch {
      // Nothing more to do; the original failure is reported.
    }
    throw error;
  }
}

interface RenameWithRetryOptions {
  from: string;
  to: string;
  fsOps: AgentSessionFs;
}

// On Windows a rename over a file another process has open fails until that handle closes.
function renameWithRetry({ from, to, fsOps }: RenameWithRetryOptions): void {
  for (let attempt = 1; ; attempt += 1) {
    try {
      fsOps.renameSync(from, to);
      return;
    } catch (error) {
      if (attempt >= RENAME_ATTEMPTS || !isRetryableRenameError(error)) throw error;
      fsOps.sleepSync(RENAME_RETRY_DELAY_MS);
    }
  }
}

function isRetryableRenameError(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'code' in error
    && typeof error.code === 'string' && RETRYABLE_RENAME_CODES.has(error.code);
}
