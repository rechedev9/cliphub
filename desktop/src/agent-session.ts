import * as fs from 'node:fs';

/** The running Studio services zv-mcp needs to reach, minted per boot. */
export interface AgentSessionTarget {
  orchestratorUrl: string;
  token: string;
  webOrigin: string;
  /** Lets an assistant cite the Studio release in a bug report. */
  studioVersion: string;
}

/** File operations the session store uses; tests replace them to simulate Windows lock failures. */
export interface AgentSessionFs {
  writeFileSync(file: string, data: string, options: { mode: number }): void;
  renameSync(from: string, to: string): void;
  rmSync(file: string, options: { force: true }): void;
  sleepSync(ms: number): void;
}

export interface AgentAccess {
  enabled: boolean;
  autoApproveStarts: boolean;
  autoApproveDeletes: boolean;
}

export interface AgentAccessChange {
  enabled: boolean;
  autoApproveStarts?: boolean;
  autoApproveDeletes?: boolean;
}

export interface AgentSessionOptions extends AgentSessionTarget, AgentAccess {
  sessionFile: string;
  fsOps?: AgentSessionFs;
}

export interface SetAgentAccessOptions extends AgentAccess {
  accessFile: string;
  sessionFile: string;
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
export function readAgentAccess(accessFile: string): AgentAccess {
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(accessFile, 'utf8'));
    if (!isRecord(parsed)) return accessOff();
    return {
      enabled: parsed.enabled === true,
      autoApproveStarts: parsed.autoApproveStarts === true,
      autoApproveDeletes: parsed.autoApproveDeletes === true,
    };
  } catch {
    return accessOff();
  }
}

export function nextAgentAccess(stored: AgentAccess, change: AgentAccessChange): AgentAccess {
  return {
    enabled: change.enabled,
    autoApproveStarts: change.autoApproveStarts ?? stored.autoApproveStarts,
    autoApproveDeletes: change.autoApproveDeletes ?? stored.autoApproveDeletes,
  };
}

function accessOff(): AgentAccess {
  return { enabled: false, autoApproveStarts: false, autoApproveDeletes: false };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** Publishes the running orchestrator to zv-mcp; the token is written only while access is on. */
export function writeAgentSession({
  sessionFile,
  enabled,
  autoApproveStarts,
  autoApproveDeletes,
  orchestratorUrl,
  token,
  webOrigin,
  studioVersion,
  fsOps = nodeFs,
}: AgentSessionOptions): void {
  if (enabled) {
    const document = { version: 1, enabled, orchestratorUrl, token, webOrigin, studioVersion, autoApproveStarts, autoApproveDeletes };
    writeFileAtomic({ file: sessionFile, document, fsOps });
    return;
  }
  try {
    writeFileAtomic({ file: sessionFile, document: { version: 1, enabled }, fsOps });
  } catch {
    removeAgentSession(sessionFile, fsOps);
  }
}

export function setAgentAccess({
  accessFile,
  sessionFile,
  enabled,
  autoApproveStarts,
  autoApproveDeletes,
  running,
  fsOps = nodeFs,
}: SetAgentAccessOptions): void {
  const next = { enabled, autoApproveStarts, autoApproveDeletes };
  const kept = keptAccess(readAgentAccess(accessFile), next);
  try {
    writeFileAtomic({ file: accessFile, document: kept, fsOps });
  } catch (error) {
    removeQuietly(accessFile, fsOps);
    if (running !== null) publishQuietly({ sessionFile, access: accessOff(), running, fsOps });
    throw error;
  }
  if (running !== null) {
    try {
      writeAgentSession({ sessionFile, ...next, ...running, fsOps });
    } catch (error) {
      removeQuietly(sessionFile, fsOps);
      throw error;
    }
  }
  if (sameAccess(kept, next)) return;
  try {
    writeFileAtomic({ file: accessFile, document: next, fsOps });
  } catch (error) {
    if (running !== null) publishQuietly({ sessionFile, access: kept, running, fsOps });
    throw error;
  }
}

function keptAccess(stored: AgentAccess, next: AgentAccess): AgentAccess {
  return {
    enabled: stored.enabled && next.enabled,
    autoApproveStarts: stored.autoApproveStarts && next.autoApproveStarts,
    autoApproveDeletes: stored.autoApproveDeletes && next.autoApproveDeletes,
  };
}

function sameAccess(a: AgentAccess, b: AgentAccess): boolean {
  return a.enabled === b.enabled && a.autoApproveStarts === b.autoApproveStarts && a.autoApproveDeletes === b.autoApproveDeletes;
}

interface PublishOptions {
  sessionFile: string;
  access: AgentAccess;
  running: AgentSessionTarget;
  fsOps: AgentSessionFs;
}

function publishQuietly({ sessionFile, access, running, fsOps }: PublishOptions): void {
  try {
    writeAgentSession({ sessionFile, ...access, ...running, fsOps });
  } catch {
    removeQuietly(sessionFile, fsOps);
  }
}

function removeQuietly(file: string, fsOps: AgentSessionFs): void {
  try {
    fsOps.rmSync(file, { force: true });
  } catch {
    return;
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
