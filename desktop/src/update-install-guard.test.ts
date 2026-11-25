import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import {
  defaultInstallGuardDeps,
  installMarkerFile,
  parseInstallMarker,
  startRecordedInstaller,
  updateInstallerRunning,
  type InstallGuardDeps,
} from './update-install-guard.ts';

const FILE = 'C:\\appdata\\cliphub-studio\\update-installing.json';
const NOW = 1_800_000_000_000;

function fakeDeps(files: Map<string, string>, alive: Set<number>): InstallGuardDeps {
  return {
    now: () => NOW,
    isAlive: (pid) => alive.has(pid),
    readFile: (file) => files.get(file) ?? null,
    writeFile: (file, text) => { files.set(file, text); },
    removeFile: (file) => { files.delete(file); },
  };
}

test('a launch while the update installer still runs is told to leave', () => {
  const files = new Map([[FILE, JSON.stringify({ pid: 4242, startedAt: NOW - 5_000 })]]);
  assert.equal(updateInstallerRunning(FILE, ['ClipHub Studio.exe'], fakeDeps(files, new Set([4242]))), true);
  assert.equal(files.has(FILE), true);
});

test('the installer relaunch, a finished installer and a stale or broken marker all start Studio', () => {
  const cases: Array<{ name: string; marker: string; argv: string[]; alive: number[] }> = [
    { name: 'installer relaunch passes --updated', marker: JSON.stringify({ pid: 4242, startedAt: NOW }), argv: ['Studio.exe', '--updated'], alive: [4242] },
    { name: 'installer exited', marker: JSON.stringify({ pid: 4242, startedAt: NOW - 5_000 }), argv: ['Studio.exe'], alive: [] },
    { name: 'marker older than ten minutes', marker: JSON.stringify({ pid: 4242, startedAt: NOW - 11 * 60_000 }), argv: ['Studio.exe'], alive: [4242] },
    { name: 'marker from the future', marker: JSON.stringify({ pid: 4242, startedAt: NOW + 60_000 }), argv: ['Studio.exe'], alive: [4242] },
    { name: 'unreadable marker', marker: '{"pid":', argv: ['Studio.exe'], alive: [4242] },
    { name: 'marker without a pid', marker: JSON.stringify({ startedAt: NOW }), argv: ['Studio.exe'], alive: [4242] },
  ];
  for (const row of cases) {
    const files = new Map([[FILE, row.marker]]);
    assert.equal(updateInstallerRunning(FILE, row.argv, fakeDeps(files, new Set(row.alive))), false, row.name);
    assert.equal(files.has(FILE), false, `${row.name} clears the marker`);
  }
});

test('no marker, a dev build or a failing file system never block Studio', () => {
  assert.equal(updateInstallerRunning(FILE, ['Studio.exe'], fakeDeps(new Map(), new Set([4242]))), false);
  assert.equal(updateInstallerRunning(null, ['Studio.exe'], fakeDeps(new Map(), new Set())), false);
  const broken = { ...fakeDeps(new Map(), new Set()), readFile: () => { throw new Error('EACCES'); } };
  assert.equal(updateInstallerRunning(FILE, ['Studio.exe'], broken), false);
  assert.equal(installMarkerFile({ isPackaged: false, getName: () => 'cliphub-studio', getPath: () => 'C:\\appdata' }), null);
  assert.equal(
    installMarkerFile({ isPackaged: true, getName: () => 'cliphub-studio', getPath: () => 'C:\\appdata' }),
    path.join('C:\\appdata', 'cliphub-studio', 'update-installing.json'),
  );
});

test('starting the installer records its pid, and a marker that cannot be written only logs', async () => {
  const files = new Map<string, string>();
  await startRecordedInstaller('C:\\updates\\setup.exe', FILE, () => assert.fail('no log'), async () => 4242, fakeDeps(files, new Set()));
  assert.deepEqual(parseInstallMarker(files.get(FILE) ?? ''), { pid: 4242, startedAt: NOW });

  const lines: string[] = [];
  const failing = { ...fakeDeps(new Map(), new Set()), writeFile: () => { throw new Error('EROFS'); } };
  await startRecordedInstaller('C:\\updates\\setup.exe', FILE, (line) => lines.push(line), async () => 4242, failing);
  assert.match(lines[0] ?? '', /install marker not written: Error: EROFS/);

  const untouched = new Map<string, string>();
  await startRecordedInstaller('C:\\updates\\setup.exe', null, () => assert.fail('no log'), async () => 4242, fakeDeps(untouched, new Set()));
  await startRecordedInstaller('C:\\updates\\setup.exe', FILE, () => assert.fail('no log'), async () => undefined, fakeDeps(untouched, new Set()));
  assert.equal(untouched.size, 0);
  await assert.rejects(startRecordedInstaller('C:\\updates\\setup.exe', FILE, () => {}, async () => { throw new Error('spawn EACCES'); }, fakeDeps(untouched, new Set())));
  assert.equal(untouched.size, 0);
});

test('the default file system and process checks round-trip a real marker', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-install-guard-'));
  try {
    const file = path.join(dir, 'profile', 'update-installing.json');
    assert.equal(defaultInstallGuardDeps.readFile(file), null);
    defaultInstallGuardDeps.writeFile(file, JSON.stringify({ pid: process.pid, startedAt: Date.now() }));
    assert.equal(defaultInstallGuardDeps.isAlive(process.pid), true);
    assert.equal(updateInstallerRunning(file, ['Studio.exe']), true);
    assert.equal(updateInstallerRunning(file, ['Studio.exe', '--updated']), false);
    assert.equal(fs.existsSync(file), false);
    assert.equal(defaultInstallGuardDeps.isAlive(2_147_483_000), false);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
