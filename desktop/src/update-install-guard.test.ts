import assert from 'node:assert/strict';
import { execFileSync, spawn } from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import {
  INSTALL_LOCK_NAME,
  installLockFiles,
  installerCopyingFiles,
  type InstallGuardFiles,
} from './update-install-guard.ts';

const BESIDE = 'C:\\Programs\\ClipHub Studio.update-installing.lock';
const INSIDE = 'C:\\Programs\\ClipHub Studio\\update-installing.lock';

function opening(codes: Record<string, string>): InstallGuardFiles {
  return {
    open: (file) => { throw Object.assign(new Error(codes[file]), { code: codes[file] }); },
    close: () => assert.fail('nothing was opened'),
    remove: () => assert.fail('a held or missing lock is never removed'),
  };
}

test('only a lock another process holds open tells a launch to leave', () => {
  const cases: Array<{ beside: string; inside: string; copying: boolean }> = [
    { beside: 'EBUSY', inside: 'ENOENT', copying: true },
    { beside: 'ENOENT', inside: 'EBUSY', copying: true },
    { beside: 'EACCES', inside: 'EBUSY', copying: true },
    { beside: 'ENOENT', inside: 'ENOENT', copying: false },
    { beside: 'EACCES', inside: 'EPERM', copying: false },
    { beside: 'ENOTDIR', inside: 'ENOENT', copying: false },
  ];
  for (const row of cases) {
    const files = opening({ [BESIDE]: row.beside, [INSIDE]: row.inside });
    assert.equal(installerCopyingFiles([BESIDE, INSIDE], files), row.copying, `${row.beside} ${row.inside}`);
  }
  assert.equal(installerCopyingFiles([], opening({})), false);
});

test('a lock nobody holds is a leftover: Studio starts and clears every one', () => {
  const calls: string[] = [];
  const files: InstallGuardFiles = {
    open: () => { calls.push('open'); return 7; },
    close: (descriptor) => { calls.push(`close ${descriptor}`); },
    remove: (file) => { calls.push(`remove ${file}`); },
  };
  assert.equal(installerCopyingFiles([BESIDE, INSIDE], files), false);
  assert.deepEqual(calls, ['open', 'close 7', `remove ${BESIDE}`, 'open', 'close 7', `remove ${INSIDE}`]);
  const stuck = { ...files, remove: () => { throw new Error('EPERM'); } };
  assert.equal(installerCopyingFiles([BESIDE, INSIDE], stuck), false);
});

test('packaged builds look beside and inside the install folder, dev builds nowhere', () => {
  const exe = path.join('C:\\Programs\\ClipHub Studio', 'ClipHub Studio.exe');
  assert.deepEqual(installLockFiles(false, exe), []);
  assert.deepEqual(installLockFiles(true, exe), [BESIDE, INSIDE]);
});

test('the installer takes a lock in whichever instance installs and drops it before it starts Studio', () => {
  const desktop = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
  const script = fs.readFileSync(path.join(desktop, 'build', 'installer.nsh'), 'utf8');
  const macro = (name: string): string => {
    const start = script.indexOf(`!macro ${name}`);
    assert.ok(start >= 0, name);
    return script.slice(start, script.indexOf('!macroend', start));
  };
  const beside = `!insertmacro takeInstallLock "$INSTDIR.${INSTALL_LOCK_NAME}"`;
  const inside = `!insertmacro takeInstallLock "$INSTDIR\\${INSTALL_LOCK_NAME}"`;
  assert.ok(macro('openInstallLock').includes('CreateFileW(w "${FILE}", i 0x40000000, i 0, p 0, i 2, i 0x04000000, p 0)'));
  const take = macro('takeInstallLock');
  assert.ok(take.indexOf('$installLock == -1') < take.lastIndexOf('!insertmacro openInstallLock'));
  const running = macro('customCheckAppRunning');
  assert.ok(running.indexOf(beside) > 0 && running.indexOf(beside) < running.indexOf('_CHECK_APP_RUNNING'));
  assert.ok(running.indexOf('!ifndef BUILD_UNINSTALLER') < running.indexOf(beside));
  const afterUninstall = macro('customUnInstallCheck');
  assert.ok(afterUninstall.indexOf('Quit') < afterUninstall.indexOf(beside));
  assert.ok(afterUninstall.indexOf(beside) < afterUninstall.indexOf(inside));
  const install = macro('customInstall');
  assert.ok(install.indexOf('CloseHandle(p $installLock)') > 0);
  assert.ok(install.indexOf('CloseHandle(p $installLock)') < install.indexOf('ExecShellAsUser'));
});

test('a real exclusive delete-on-close handle blocks the launch until its process ends', { skip: process.platform !== 'win32' }, async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-install-guard-'));
  const locks = installLockFiles(true, path.join(dir, 'ClipHub Studio.exe'));
  const file = locks[1] ?? '';
  const holder = spawn('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command',
    `$s = [IO.FileStream]::new('${file}', 'Create', 'Write', 'None', 1, 'DeleteOnClose'); [Console]::Out.WriteLine('held'); [Console]::In.ReadLine() | Out-Null; $s.Dispose()`,
  ], { stdio: ['pipe', 'pipe', 'ignore'] });
  try {
    await new Promise<void>((resolve, reject) => {
      holder.stdout.once('data', () => resolve());
      holder.once('error', reject);
      holder.once('exit', () => reject(new Error('holder exited early')));
    });
    assert.equal(installerCopyingFiles(locks), true);
    assert.equal(fs.existsSync(file), true);
    execFileSync('taskkill', ['/PID', String(holder.pid), '/F'], { stdio: 'ignore' });
    await new Promise((resolve) => holder.once('close', resolve));
    assert.equal(installerCopyingFiles(locks), false);
    assert.equal(fs.existsSync(file), false);
    fs.writeFileSync(file, '');
    assert.equal(installerCopyingFiles(locks), false);
    assert.equal(fs.existsSync(file), false);
  } finally {
    holder.kill();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
