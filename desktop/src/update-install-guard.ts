import * as fs from 'node:fs';
import * as path from 'node:path';

export const INSTALL_LOCK_NAME = 'update-installing.lock';

export interface InstallGuardApp {
  readonly isPackaged: boolean;
  getName(): string;
  getPath(name: 'appData'): string;
}

export interface InstallGuardFiles {
  open(file: string): number;
  close(descriptor: number): void;
  remove(file: string): void;
}

export const defaultInstallGuardFiles: InstallGuardFiles = {
  open: (file) => fs.openSync(file, 'r'),
  close: (descriptor) => fs.closeSync(descriptor),
  remove: (file) => fs.rmSync(file, { force: true }),
};

export function installLockFile(app: InstallGuardApp): string | null {
  return app.isPackaged ? path.join(app.getPath('appData'), app.getName(), INSTALL_LOCK_NAME) : null;
}

export function installerCopyingFiles(
  file: string | null,
  files: InstallGuardFiles = defaultInstallGuardFiles,
): boolean {
  if (file === null) return false;
  let descriptor: number;
  try {
    descriptor = files.open(file);
  } catch (err) {
    return (err as NodeJS.ErrnoException).code === 'EBUSY';
  }
  try {
    files.close(descriptor);
    files.remove(file);
  } catch {
    return false;
  }
  return false;
}
