import assert from 'node:assert/strict';
import { execFileSync, spawn } from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import {
  INSTALL_LOCK_NAME,
  installLockFile,
  installerCopyingFiles,
  type InstallGuardFiles,
} from './update-install-guard.ts';

const FILE = 'C:\\appdata\\cliphub-studio\\update-installing.lock';

function failingOpen(code: string): InstallGuardFiles {
  return {
    open: () => { throw Object.assign(new Error(code), { code }); },
    close: () => assert.fail('nothing was opened'),
    remove: () => assert.fail('a held or missing lock is never removed'),
  };
}

test('only a lock another process holds open tells a launch to leave', () => {
  const cases: Array<{ code: string; copying: boolean }> = [
    { code: 'EBUSY', copying: true },
    { code: 'ENOENT', copying: false },
    { code: 'EACCES', copying: false },
    { code: 'EPERM', copying: false },
    { code: 'ENOTDIR', copying: false },
  ];
  for (const row of cases) {
    assert.equal(installerCopyingFiles(FILE, failingOpen(row.code)), row.copying, row.code);
  }
  assert.equal(installerCopyingFiles(null, failingOpen('EBUSY')), false);
});

test('a lock nobody holds is a leftover: Studio starts and clears it', () => {
  const calls: string[] = [];
  const files: InstallGuardFiles = {
    open: () => { calls.push('open'); return 7; },
    close: (descriptor) => { calls.push(`close ${descriptor}`); },
    remove: (file) => { calls.push(`remove ${file}`); },
  };
  assert.equal(installerCopyingFiles(FILE, files), false);
  assert.deepEqual(calls, ['open', 'close 7', `remove ${FILE}`]);
  const stuck = { ...files, remove: () => { throw new Error('EPERM'); } };
  assert.equal(installerCopyingFiles(FILE, stuck), false);
});

test('the lock lives in the canonical profile of packaged builds only', () => {
  const app = (isPackaged: boolean) => ({ isPackaged, getName: () => 'cliphub-studio', getPath: () => 'C:\\appdata' });
  assert.equal(installLockFile(app(false)), null);
  assert.equal(installLockFile(app(true)), path.join('C:\\appdata', 'cliphub-studio', INSTALL_LOCK_NAME));
});

test('the installer takes the lock before it replaces files and drops it before it starts Studio', () => {
  const desktop = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
  const script = fs.readFileSync(path.join(desktop, 'build', 'installer.nsh'), 'utf8');
  const manifest = JSON.parse(fs.readFileSync(path.join(desktop, 'package.json'), 'utf8'));
  assert.equal(manifest.name, 'cliphub-studio');
  const check = script.slice(script.indexOf('!macro customCheckAppRunning'));
  assert.ok(check.includes('"$R8\\${APP_PACKAGE_NAME}\\' + INSTALL_LOCK_NAME + '", i 0x40000000, i 0, p 0, i 2, i 0x04000000, p 0)'));
  assert.ok(check.indexOf('CreateFileW') < check.indexOf('_CHECK_APP_RUNNING'));
  const install = script.slice(script.indexOf('!macro customInstall'), script.indexOf('!macro customCheckAppRunning'));
  assert.ok(install.indexOf('CloseHandle(p $installLock)') > 0);
  assert.ok(install.indexOf('CloseHandle(p $installLock)') < install.indexOf('ExecShellAsUser'));
});

test('a real exclusive delete-on-close handle blocks the launch until its process ends', { skip: process.platform !== 'win32' }, async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-install-guard-'));
  const file = path.join(dir, INSTALL_LOCK_NAME);
  const holder = spawn('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command',
    `$s = [IO.FileStream]::new('${file}', 'Create', 'Write', 'None', 1, 'DeleteOnClose'); [Console]::Out.WriteLine('held'); [Console]::In.ReadLine() | Out-Null; $s.Dispose()`,
  ], { stdio: ['pipe', 'pipe', 'ignore'] });
  try {
    await new Promise<void>((resolve, reject) => {
      holder.stdout.once('data', () => resolve());
      holder.once('error', reject);
      holder.once('exit', () => reject(new Error('holder exited early')));
    });
    assert.equal(installerCopyingFiles(file), true);
    assert.equal(fs.existsSync(file), true);
    execFileSync('taskkill', ['/PID', String(holder.pid), '/F'], { stdio: 'ignore' });
    await new Promise((resolve) => holder.once('close', resolve));
    assert.equal(installerCopyingFiles(file), false);
    assert.equal(fs.existsSync(file), false);
    fs.writeFileSync(file, '');
    assert.equal(installerCopyingFiles(file), false);
    assert.equal(fs.existsSync(file), false);
  } finally {
    holder.kill();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
