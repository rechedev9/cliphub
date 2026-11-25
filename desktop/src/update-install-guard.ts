import * as fs from 'node:fs';
import * as path from 'node:path';

const MARKER_NAME = 'update-installing.json';
const MARKER_MAX_AGE_MS = 10 * 60 * 1000;

export const INSTALLER_LAUNCH_ARG = '--updated';

export interface InstallMarker {
  pid: number;
  startedAt: number;
}

export interface InstallGuardApp {
  readonly isPackaged: boolean;
  getName(): string;
  getPath(name: 'appData'): string;
}

export interface InstallGuardDeps {
  now(): number;
  isAlive(pid: number): boolean;
  readFile(file: string): string | null;
  writeFile(file: string, text: string): void;
  removeFile(file: string): void;
}

function processAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return (err as NodeJS.ErrnoException).code === 'EPERM';
  }
}

function readIfPresent(file: string): string | null {
  try {
    return fs.readFileSync(file, 'utf8');
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return null;
    throw err;
  }
}

export const defaultInstallGuardDeps: InstallGuardDeps = {
  now: () => Date.now(),
  isAlive: processAlive,
  readFile: readIfPresent,
  writeFile: (file, text) => {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, text);
  },
  removeFile: (file) => fs.rmSync(file, { force: true }),
};

export function installMarkerFile(app: InstallGuardApp): string | null {
  return app.isPackaged ? path.join(app.getPath('appData'), app.getName(), MARKER_NAME) : null;
}

export function parseInstallMarker(text: string): InstallMarker | null {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null) return null;
  const { pid, startedAt } = value as Record<string, unknown>;
  if (typeof pid !== 'number' || !Number.isSafeInteger(pid) || pid <= 0) return null;
  if (typeof startedAt !== 'number' || !Number.isFinite(startedAt)) return null;
  return { pid, startedAt };
}

export function updateInstallerRunning(
  file: string | null,
  argv: readonly string[],
  deps: InstallGuardDeps = defaultInstallGuardDeps,
): boolean {
  if (file === null) return false;
  try {
    if (argv.includes(INSTALLER_LAUNCH_ARG)) {
      deps.removeFile(file);
      return false;
    }
    const text = deps.readFile(file);
    if (text === null) return false;
    const marker = parseInstallMarker(text);
    const age = marker === null ? -1 : deps.now() - marker.startedAt;
    if (marker !== null && age >= 0 && age <= MARKER_MAX_AGE_MS && deps.isAlive(marker.pid)) return true;
    deps.removeFile(file);
    return false;
  } catch {
    return false;
  }
}

export async function startRecordedInstaller(
  installerPath: string,
  file: string | null,
  log: (line: string) => void,
  spawnInstaller: (installerPath: string) => Promise<number | undefined>,
  deps: InstallGuardDeps = defaultInstallGuardDeps,
): Promise<void> {
  const pid = await spawnInstaller(installerPath);
  if (file === null || pid === undefined) return;
  try {
    deps.writeFile(file, JSON.stringify({ pid, startedAt: deps.now() }));
  } catch (err) {
    log(`[update] install marker not written: ${String(err)}\n`);
  }
}
