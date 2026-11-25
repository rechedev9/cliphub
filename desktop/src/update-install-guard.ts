import * as fs from 'node:fs';
import * as path from 'node:path';

export const INSTALL_LOCK_NAME = 'update-installing.lock';

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

export function installLockFiles(isPackaged: boolean, execPath: string): string[] {
  if (!isPackaged) return [];
  const installDirectory = path.dirname(execPath);
  return [`${installDirectory}.${INSTALL_LOCK_NAME}`, path.join(installDirectory, INSTALL_LOCK_NAME)];
}

export function installerCopyingFiles(
  lockFiles: readonly string[],
  files: InstallGuardFiles = defaultInstallGuardFiles,
): boolean {
  return lockFiles.filter((file) => lockHeld(file, files)).length > 0;
}

function lockHeld(file: string, files: InstallGuardFiles): boolean {
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
