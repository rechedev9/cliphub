import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import {
  requestCanonicalSingleInstanceLock,
  type UserDataLockApp,
} from './user-data-lock.ts';

function temporaryRoot(t: test.TestContext): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'fragforge-user-data-lock-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  return root;
}

class FakeUserDataLockApp implements UserDataLockApp {
  readonly isPackaged: boolean;
  readonly appData: string;
  readonly lockResult: boolean | Error;
  readonly setPathCalls: string[] = [];
  readonly lockPaths: string[] = [];
  currentUserData: string;

  constructor(
    isPackaged: boolean,
    appData: string,
    profileUserData: string,
    lockResult: boolean | Error,
  ) {
    this.isPackaged = isPackaged;
    this.appData = appData;
    this.currentUserData = profileUserData;
    this.lockResult = lockResult;
  }

  getName(): string {
    return 'FragForge Studio';
  }

  getPath(name: 'appData' | 'userData'): string {
    return name === 'appData' ? this.appData : this.currentUserData;
  }

  setPath(name: 'userData', value: string): void {
    assert.equal(name, 'userData');
    assert.equal(fs.lstatSync(value).isDirectory(), true);
    this.setPathCalls.push(value);
    this.currentUserData = value;
  }

  requestSingleInstanceLock(): boolean {
    this.lockPaths.push(this.currentUserData);
    if (this.lockResult instanceof Error) throw this.lockResult;
    return this.lockResult;
  }
}

test('creates fresh canonical and explicit profile directories before setPath', (t) => {
  const root = temporaryRoot(t);
  const appData = path.join(root, 'app-data');
  const profileUserData = path.join(root, 'profiles', 'fresh');
  const canonicalUserData = path.join(appData, 'FragForge Studio');
  const app = new FakeUserDataLockApp(true, appData, profileUserData, true);

  assert.equal(requestCanonicalSingleInstanceLock(app), true);

  assert.equal(fs.lstatSync(canonicalUserData).isDirectory(), true);
  assert.equal(fs.lstatSync(profileUserData).isDirectory(), true);
  assert.deepEqual(app.lockPaths, [canonicalUserData]);
  assert.deepEqual(app.setPathCalls, [canonicalUserData, profileUserData]);
  assert.equal(app.currentUserData, profileUserData);
});

test('restores a fresh explicit profile when lock acquisition throws', (t) => {
  const root = temporaryRoot(t);
  const profileUserData = path.join(root, 'profiles', 'restored-after-error');
  const lockError = new Error('lock failed');
  const app = new FakeUserDataLockApp(
    true,
    path.join(root, 'app-data'),
    profileUserData,
    lockError,
  );

  assert.throws(
    () => requestCanonicalSingleInstanceLock(app),
    (error) => error === lockError,
  );

  assert.equal(fs.lstatSync(profileUserData).isDirectory(), true);
  assert.equal(app.currentUserData, profileUserData);
  assert.equal(app.setPathCalls.at(-1), profileUserData);
});

test('keeps development and e2e locks scoped to their existing profile', (t) => {
  const root = temporaryRoot(t);
  const profileUserData = path.join(root, 'isolated-e2e-profile');
  const app = new FakeUserDataLockApp(
    false,
    path.join(root, 'app-data'),
    profileUserData,
    true,
  );

  assert.equal(requestCanonicalSingleInstanceLock(app), true);

  assert.deepEqual(app.lockPaths, [profileUserData]);
  assert.deepEqual(app.setPathCalls, []);
  assert.equal(fs.existsSync(profileUserData), false);
});

test('rejects a non-directory canonical lock namespace before acquiring the lock', (t) => {
  const root = temporaryRoot(t);
  const appData = path.join(root, 'app-data');
  const canonicalUserData = path.join(appData, 'FragForge Studio');
  fs.mkdirSync(appData);
  fs.writeFileSync(canonicalUserData, 'not a directory');
  const app = new FakeUserDataLockApp(
    true,
    appData,
    path.join(root, 'profile'),
    true,
  );

  assert.throws(
    () => requestCanonicalSingleInstanceLock(app),
    /Could not prepare canonical Electron userData directory/,
  );
  assert.deepEqual(app.lockPaths, []);
  assert.deepEqual(app.setPathCalls, []);
});
