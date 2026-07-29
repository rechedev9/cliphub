import * as fs from 'node:fs';
import * as path from 'node:path';

export interface UserDataLockApp {
  readonly isPackaged: boolean;
  getName(): string;
  getPath(name: 'appData' | 'userData'): string;
  setPath(name: 'userData', value: string): void;
  requestSingleInstanceLock(): boolean;
}

function prepareUserDataDirectory(directory: string, purpose: string): void {
  try {
    fs.mkdirSync(directory, { recursive: true });
    if (!fs.statSync(directory).isDirectory()) {
      throw new Error('path is not a directory');
    }
  } catch (cause) {
    throw new Error(
      `Could not prepare ${purpose} Electron userData directory "${directory}"`,
      { cause },
    );
  }
}

/**
 * Packaged instances share one canonical lock namespace even when launched
 * with an explicit profile. Electron requires every setPath target to exist.
 */
export function requestCanonicalSingleInstanceLock(app: UserDataLockApp): boolean {
  const profileUserData = app.getPath('userData');
  if (!app.isPackaged) {
    return app.requestSingleInstanceLock();
  }

  const canonicalUserData = path.join(app.getPath('appData'), app.getName());
  prepareUserDataDirectory(canonicalUserData, 'canonical');
  app.setPath('userData', canonicalUserData);

  try {
    return app.requestSingleInstanceLock();
  } finally {
    prepareUserDataDirectory(profileUserData, 'profile');
    app.setPath('userData', profileUserData);
  }
}
