import * as fs from 'node:fs';
import * as net from 'node:net';

export interface StablePortOptions {
  host: string;
  portsFile: string;
  logLine: (line: string) => void;
  signal?: AbortSignal;
  isPortFree?: (port: number, host: string) => Promise<boolean>;
  allocateFreePort?: (host: string) => Promise<number>;
}

export type StableStudioPortOptions = StablePortOptions;

type ServiceKey = 'web';

interface SavedPortOptions {
  host: string;
  logLine: (line: string) => void;
  isPortFree: (port: number, host: string) => Promise<boolean>;
}

const MAX_ALLOCATION_ATTEMPTS = 32;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isValidPort(value: unknown): value is number {
  return typeof value === 'number'
    && Number.isInteger(value)
    && value >= 1
    && value <= 65_535;
}

function originChangeHint(): string {
  return ' the reel library kept in the browser localStorage is keyed by origin, so it may appear empty on the new port';
}

function throwIfAborted(signal: AbortSignal | undefined): void {
  if (signal?.aborted) throw new Error('port allocation aborted');
}

function readSavedPorts(portsFile: string): Record<string, unknown> {
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(portsFile, 'utf8'));
    return isRecord(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

async function reusableSavedPort(
  key: ServiceKey,
  saved: Record<string, unknown>,
  selected: Set<number>,
  options: SavedPortOptions,
): Promise<number | undefined> {
  const savedPort = saved[key];
  if (!isValidPort(savedPort)) return undefined;

  if (selected.has(savedPort)) {
    options.logLine(
      `[ports] saved ${key} port ${savedPort} conflicts with another service, picking a new one;${originChangeHint()}\n`,
    );
    return undefined;
  }
  if (await options.isPortFree(savedPort, options.host)) {
    selected.add(savedPort);
    return savedPort;
  }

  options.logLine(
    `[ports] saved ${key} port ${savedPort} was taken, picking a new one;${originChangeHint()}\n`,
  );
  return undefined;
}

async function allocateDistinctPort(
  selected: Set<number>,
  host: string,
  allocateFreePort: (host: string) => Promise<number>,
  signal: AbortSignal | undefined,
): Promise<number> {
  for (let attempt = 0; attempt < MAX_ALLOCATION_ATTEMPTS; attempt += 1) {
    throwIfAborted(signal);
    const port = await allocateFreePort(host);
    throwIfAborted(signal);
    if (!isValidPort(port)) {
      throw new Error(`free port allocator returned invalid port ${String(port)}`);
    }
    if (!selected.has(port)) {
      selected.add(port);
      return port;
    }
  }
  throw new Error('could not allocate distinct desktop service ports');
}

function persistPorts(
  saved: Record<string, unknown>,
  portsFile: string,
  logLine: (line: string) => void,
): void {
  const temporary = `${portsFile}.tmp`;
  let descriptor: number | undefined;
  try {
    fs.rmSync(temporary, { force: true });
    descriptor = fs.openSync(temporary, 'w', 0o600);
    fs.writeFileSync(descriptor, JSON.stringify(saved));
    fs.fsyncSync(descriptor);
    fs.closeSync(descriptor);
    descriptor = undefined;
    fs.renameSync(temporary, portsFile);
  } catch (err) {
    if (descriptor !== undefined) {
      try {
        fs.closeSync(descriptor);
      } catch {
        // The original publication error below is the useful diagnostic.
      }
    }
    try {
      fs.rmSync(temporary, { force: true });
    } catch {
      // Best-effort cleanup must not replace the original publication error.
    }
    logLine(`[ports] could not persist service ports: ${String(err)}\n`);
  }
}

/** Chooses the single Studio port, preferring the former web origin. */
export async function allocateStableStudioPort({
  host,
  portsFile,
  logLine,
  signal,
  isPortFree = loopbackPortFree,
  allocateFreePort = allocateLoopbackPort,
}: StableStudioPortOptions): Promise<number> {
  throwIfAborted(signal);
  const saved = readSavedPorts(portsFile);
  let changed = false;
  if ('discovery_secret' in saved) {
    delete saved.discovery_secret;
    changed = true;
  }
  if ('orchestrator' in saved) {
    delete saved.orchestrator;
    changed = true;
  }
  const selected = new Set<number>();
  let port = await reusableSavedPort('web', saved, selected, { host, logLine, isPortFree });
  throwIfAborted(signal);
  if (port === undefined) {
    port = await allocateDistinctPort(selected, host, allocateFreePort, signal);
    saved.web = port;
    changed = true;
  }
  if (changed) persistPorts(saved, portsFile, logLine);
  return port;
}

/** Grabs an OS-assigned free loopback port, then releases it for the child. */
function allocateLoopbackPort(host: string): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.once('error', reject);
    server.listen(0, host, () => {
      const address = server.address();
      if (address === null || typeof address === 'string') {
        server.close(() => reject(new Error('free port server has no assigned address')));
        return;
      }
      const { port } = address;
      server.close(() => resolve(port));
    });
  });
}

/** Reports whether a specific loopback port is currently free. */
function loopbackPortFree(port: number, host: string): Promise<boolean> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.unref();
    server.once('error', () => resolve(false));
    server.listen(port, host, () => server.close(() => resolve(true)));
  });
}
