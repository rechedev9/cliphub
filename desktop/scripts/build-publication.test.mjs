import assert from 'node:assert/strict';
import {
  existsSync,
  linkSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import test from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

const isWin32 = process.platform === 'win32';
/** PowerShell publication harness is Windows-only. */
const winTest = isWin32 ? test : test.skip;

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const helperPath = join(repoRoot, 'scripts', 'build-publication.ps1');

function writeSet(root, values) {
  mkdirSync(root, { recursive: true });
  for (const [name, value] of Object.entries(values)) {
    writeFileSync(join(root, `${name}.exe`), value);
  }
}

function writePublicationHarness({
  bin,
  stage,
  backup,
  names = ['a', 'b'],
  injectedMove = '',
  injectedRemove = '',
  injectedRemoveDirectory = '',
}) {
  const harness = join(dirname(bin), `publish-${Date.now()}-${Math.random()}.ps1`);
  const publicationNames = names
    .map((name) => `'${name.replaceAll("'", "''")}'`)
    .join(', ');
  writeFileSync(harness, `
$ErrorActionPreference = "Stop"
. '${helperPath.replaceAll("'", "''")}'
$move = {
  param([string]$From, [string]$To)
  ${injectedMove}
  Move-BuildPublicationFileDurably -From $From -To $To
}
$remove = {
  param([string]$Path)
  ${injectedRemove}
  Remove-Item -LiteralPath $Path -Force
}
$removeDirectory = {
  param([string]$Path)
  ${injectedRemoveDirectory}
  Remove-Item -LiteralPath $Path -Recurse -Force
}
Publish-BuildArtifacts -Names @(${publicationNames}) -BinDir '${bin.replaceAll("'", "''")}' -StagingDir '${stage.replaceAll("'", "''")}' -BackupDir '${backup.replaceAll("'", "''")}' -MoveFile $move -RemoveFile $remove -RemoveDirectory $removeDirectory
`);
  return harness;
}

function invokePublication(options) {
  const harness = writePublicationHarness(options);
  const result = spawnSync('powershell.exe', ['-NoProfile', '-File', harness], {
    encoding: 'utf8',
  });
  rmSync(harness, { force: true });
  return result;
}

function startPublication(options) {
  const harness = writePublicationHarness(options);
  const child = spawn('powershell.exe', ['-NoProfile', '-File', harness]);
  let stdout = '';
  let stderr = '';
  child.stdout.setEncoding('utf8');
  child.stderr.setEncoding('utf8');
  child.stdout.on('data', (chunk) => {
    stdout += chunk;
  });
  child.stderr.on('data', (chunk) => {
    stderr += chunk;
  });
  const completion = new Promise((resolveCompletion, rejectCompletion) => {
    child.once('error', rejectCompletion);
    child.once('close', (status) => {
      rmSync(harness, { force: true });
      resolveCompletion({ status, stdout, stderr });
    });
  });
  return { child, completion };
}

function invokeRecovery(bin, {
  injectedMove = '',
  injectedRemove = '',
  injectedRemoveDirectory = '',
} = {}) {
  const harness = join(dirname(bin), `recover-${Date.now()}-${Math.random()}.ps1`);
  writeFileSync(harness, `
$ErrorActionPreference = "Stop"
. '${helperPath.replaceAll("'", "''")}'
$move = {
  param([string]$From, [string]$To)
  ${injectedMove}
  Move-BuildPublicationFileDurably -From $From -To $To
}
$remove = {
  param([string]$Path)
  ${injectedRemove}
  Remove-Item -LiteralPath $Path -Force
}
$removeDirectory = {
  param([string]$Path)
  ${injectedRemoveDirectory}
  Remove-Item -LiteralPath $Path -Recurse -Force
}
Recover-BuildPublication -BinDir '${bin.replaceAll("'", "''")}' -MoveFile $move -RemoveFile $remove -RemoveDirectory $removeDirectory
`);
  const result = spawnSync('powershell.exe', ['-NoProfile', '-File', harness], {
    encoding: 'utf8',
  });
  rmSync(harness, { force: true });
  return result;
}

function invokeLock(bin) {
  const harness = join(dirname(bin), `lock-${Date.now()}-${Math.random()}.ps1`);
  writeFileSync(harness, `
$ErrorActionPreference = "Stop"
. '${helperPath.replaceAll("'", "''")}'
$lock = $null
try {
  $lock = Enter-BuildPublicationLock -BinDir '${bin.replaceAll("'", "''")}'
} finally {
  if ($null -ne $lock) {
    $lock.Dispose()
  }
}
`);
  const result = spawnSync('powershell.exe', ['-NoProfile', '-File', harness], {
    encoding: 'utf8',
  });
  rmSync(harness, { force: true });
  return result;
}

async function waitForPath(path, timeoutMs = 5000) {
  const deadline = Date.now() + timeoutMs;
  while (!existsSync(path)) {
    if (Date.now() >= deadline) {
      throw new Error(`timed out waiting for ${path}`);
    }
    await delay(20);
  }
}

function createDirectoryReparsePoint(t, target, path, type) {
  try {
    symlinkSync(target, path, type);
    return true;
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error &&
        ['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) {
      t.skip(`directory reparse points are unavailable: ${error.code}`);
      return false;
    }
    throw error;
  }
}

winTest('build publication uses a write-through Win32 move primitive', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-durable-move-'));
  const sourcePath = join(root, 'source.bin');
  const destinationPath = join(root, 'destination.bin');
  const harness = join(root, 'move.ps1');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeFileSync(sourcePath, 'durable-content');
  writeFileSync(harness, `
$ErrorActionPreference = "Stop"
. '${helperPath.replaceAll("'", "''")}'
Move-BuildPublicationFileDurably -From '${sourcePath.replaceAll("'", "''")}' -To '${destinationPath.replaceAll("'", "''")}'
`);

  const result = spawnSync('powershell.exe', ['-NoProfile', '-File', harness], {
    encoding: 'utf8',
  });
  const helper = readFileSync(helperPath, 'utf8');

  assert.equal(result.status, 0, result.stderr);
  assert.equal(existsSync(sourcePath), false);
  assert.equal(readFileSync(destinationPath, 'utf8'), 'durable-content');
  assert.match(helper, /MoveFileExW/);
  assert.match(helper, /MOVEFILE_WRITE_THROUGH/);
});

winTest('build publication lock rejects a symbolic link without modifying its target', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-lock-symlink-'));
  const bin = join(root, 'bin');
  const linkedTarget = join(root, 'linked-lock-target.txt');
  const lockPath = join(bin, '.build-publication.lock');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(bin, { recursive: true });
  writeFileSync(linkedTarget, 'do-not-truncate');
  try {
    symlinkSync(linkedTarget, lockPath, 'file');
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error &&
        ['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) {
      t.skip(`symbolic links are unavailable: ${error.code}`);
      return;
    }
    throw error;
  }

  const result = invokeLock(bin);

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /lock path must be an unlinked\s+regular file/);
  assert.equal(lstatSync(lockPath).isSymbolicLink(), true);
  assert.equal(readFileSync(linkedTarget, 'utf8'), 'do-not-truncate');
});

winTest('build publication lock rejects a hard link without modifying its target', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-lock-hardlink-'));
  const bin = join(root, 'bin');
  const linkedTarget = join(root, 'linked-lock-target.txt');
  const lockPath = join(bin, '.build-publication.lock');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(bin, { recursive: true });
  writeFileSync(linkedTarget, 'do-not-truncate');
  linkSync(linkedTarget, lockPath);

  const result = invokeLock(bin);

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /lock path must be an unlinked\s+regular file/);
  assert.equal(readFileSync(linkedTarget, 'utf8'), 'do-not-truncate');
  assert.equal(readFileSync(lockPath, 'utf8'), 'do-not-truncate');
});

winTest('initial journal is durable before the first artifact move', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-initial-journal-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a90');
  const backup = join(bin, '.backup-b90');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a' });
  writeSet(stage, { a: 'new-a' });

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a'],
    injectedMove: `
if ($From -eq '${join(bin, 'a.exe').replaceAll("'", "''")}') {
  if (-not (Test-Path -LiteralPath '${journal.replaceAll("'", "''")}')) {
    throw "initial journal was not published"
  }
  $document = Get-Content -LiteralPath '${journal.replaceAll("'", "''")}' -Raw | ConvertFrom-Json
  if ($document.Phase -ne "publishing" -or $document.Items[0].Phase -ne "pending") {
    throw "initial journal did not precede the artifact move"
  }
}
`,
  });

  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
});

winTest('build artifact publication commits a complete set and cleans its backup', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-publish-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const result = invokePublication({ bin, stage, backup });

  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(existsSync(backup), false);
});

winTest('build artifact publication rejects case-insensitive duplicate names before journal or moves', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-duplicate-name-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  const moveMarker = join(root, 'move-was-called');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a' });
  writeSet(stage, { a: 'new-a' });

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a', 'A'],
    injectedMove: `[System.IO.File]::WriteAllText('${moveMarker.replaceAll("'", "''")}', 'called')`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /duplicate build artifact name: A/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(stage, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(moveMarker), false);
});

winTest('build artifact publication rejects a directory target before transaction side effects', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-directory-target-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  const target = join(bin, 'a.exe');
  const targetMarker = join(target, 'untouched.txt');
  const moveMarker = join(root, 'move-was-called');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(stage, { a: 'new-a' });
  mkdirSync(target, { recursive: true });
  writeFileSync(targetMarker, 'directory-target');

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a'],
    injectedMove: `[System.IO.File]::WriteAllText('${moveMarker.replaceAll("'", "''")}', 'called')`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /build artifact target is not a regular file/);
  assert.equal(lstatSync(target).isDirectory(), true);
  assert.equal(readFileSync(targetMarker, 'utf8'), 'directory-target');
  assert.equal(readFileSync(join(stage, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(moveMarker), false);
});

winTest('build artifact publication rejects a symbolic-link target before transaction side effects', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-symlink-target-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  const target = join(bin, 'a.exe');
  const source = join(root, 'source.exe');
  const moveMarker = join(root, 'move-was-called');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(stage, { a: 'new-a' });
  writeFileSync(source, 'symlink-target');
  try {
    symlinkSync(source, target, 'file');
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error &&
        ['EPERM', 'EACCES', 'UNKNOWN'].includes(error.code)) {
      t.skip(`symbolic links are unavailable: ${error.code}`);
      return;
    }
    throw error;
  }

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a'],
    injectedMove: `[System.IO.File]::WriteAllText('${moveMarker.replaceAll("'", "''")}', 'called')`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /build artifact target is not a regular file/);
  assert.equal(lstatSync(target).isSymbolicLink(), true);
  assert.equal(readFileSync(source, 'utf8'), 'symlink-target');
  assert.equal(readFileSync(join(stage, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(moveMarker), false);
});

winTest('build artifact publication rejects a staging junction without touching its external target', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-stage-junction-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-stage1');
  const backup = join(bin, '.backup-backup1');
  const externalStage = join(root, 'external-stage');
  const externalArtifact = join(externalStage, 'a.exe');
  const externalMarker = join(externalStage, 'keep.bin');
  const artifactBytes = Buffer.from([0x00, 0x11, 0x7f, 0x80, 0xff]);
  const markerBytes = Buffer.from([0xde, 0xad, 0x00, 0xbe, 0xef]);
  const moveMarker = join(root, 'move-was-called');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a' });
  mkdirSync(externalStage, { recursive: true });
  writeFileSync(externalArtifact, artifactBytes);
  writeFileSync(externalMarker, markerBytes);
  if (!createDirectoryReparsePoint(t, externalStage, stage, 'junction')) {
    return;
  }

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a'],
    injectedMove: `[System.IO.File]::WriteAllText('${moveMarker.replaceAll("'", "''")}', 'called')`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /unsafe build publication transaction directory/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.deepEqual(readFileSync(externalArtifact), artifactBytes);
  assert.deepEqual(readFileSync(externalMarker), markerBytes);
  assert.equal(lstatSync(stage).isSymbolicLink(), true);
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(moveMarker), false);
});

winTest('build artifact publication rejects a backup symlink without touching its external target', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-backup-symlink-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-stage2');
  const backup = join(bin, '.backup-backup2');
  const externalBackup = join(root, 'external-backup');
  const externalArtifact = join(externalBackup, 'a.exe');
  const externalMarker = join(externalBackup, 'keep.bin');
  const artifactBytes = Buffer.from([0xff, 0x80, 0x7f, 0x11, 0x00]);
  const markerBytes = Buffer.from([0xca, 0xfe, 0x00, 0xba, 0xbe]);
  const moveMarker = join(root, 'move-was-called');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a' });
  writeSet(stage, { a: 'new-a' });
  mkdirSync(externalBackup, { recursive: true });
  writeFileSync(externalArtifact, artifactBytes);
  writeFileSync(externalMarker, markerBytes);
  if (!createDirectoryReparsePoint(t, externalBackup, backup, 'dir')) {
    return;
  }

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a'],
    injectedMove: `[System.IO.File]::WriteAllText('${moveMarker.replaceAll("'", "''")}', 'called')`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /unsafe build publication transaction directory/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(stage, 'a.exe'), 'utf8'), 'new-a');
  assert.deepEqual(readFileSync(externalArtifact), artifactBytes);
  assert.deepEqual(readFileSync(externalMarker), markerBytes);
  assert.equal(lstatSync(backup).isSymbolicLink(), true);
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
  assert.equal(existsSync(moveMarker), false);
});

winTest('build artifact publication fully rolls back a failed staged move', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-rollback-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const result = invokePublication({
    bin,
    stage,
    backup,
    injectedMove: `if ($From -eq '${join(stage, 'b.exe').replaceAll("'", "''")}') { throw "injected publish failure" }`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /injected publish failure/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(backup), false);
});

winTest('build artifact publication retains recovery backup when rollback is incomplete', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-recovery-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const failedPublish = join(stage, 'b.exe').replaceAll("'", "''");
  const failedRestore = join(backup, 'a.exe').replaceAll("'", "''");
  const result = invokePublication({
    bin,
    stage,
    backup,
    injectedMove: `
if ($From -eq '${failedPublish}') { throw "injected publish failure" }
if ($From -eq '${failedRestore}') { throw "injected restore failure" }
`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /rollback was incomplete/);
  assert.match(result.stderr, /Recovery artifacts and journal were retained/);
  assert.equal(readFileSync(join(backup, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
});

winTest('build artifact publication retains a new target when its original backup disappears', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-missing-backup-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const failedPublish = join(stage, 'b.exe').replaceAll("'", "''");
  const missingBackup = join(backup, 'a.exe').replaceAll("'", "''");
  const result = invokePublication({
    bin,
    stage,
    backup,
    injectedMove: `
if ($From -eq '${failedPublish}') {
  Remove-Item -LiteralPath '${missingBackup}' -Force
  throw "injected missing backup"
}
`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /rollback was incomplete/);
  assert.match(result.stderr, /original backup is missing after phase 'published'; target was retained/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(backup), true);
});

winTest('build artifact publication recovers when a backup move completes before throwing', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-ambiguous-move-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-test');
  const backup = join(bin, '.backup-test');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const ambiguousSource = join(bin, 'a.exe').replaceAll("'", "''");
  const result = invokePublication({
    bin,
    stage,
    backup,
    injectedMove: `
if ($From -eq '${ambiguousSource}') {
  [System.IO.File]::Move($From, $To)
  throw "injected post-move failure"
}
`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /injected post-move failure/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(backup), false);
});

winTest('recovery retains journal and target when a durable phase says the original was moved', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-missing-durable-backup-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a70');
  const backup = join(bin, '.backup-b70');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'candidate-a' });
  mkdirSync(stage, { recursive: true });
  mkdirSync(backup, { recursive: true });
  writeFileSync(journal, JSON.stringify({
    SchemaVersion: 2,
    Phase: 'publishing',
    BackupDirectory: backup,
    StagingDirectory: stage,
    Items: [{
      Name: 'a',
      Target: join(bin, 'a.exe'),
      Backup: join(backup, 'a.exe'),
      HadOriginal: true,
      Phase: 'backup_created',
    }],
  }));

  const result = invokeRecovery(bin);

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /original backup is missing\s+after phase 'backup_created'/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'candidate-a');
  assert.equal(existsSync(journal), true);
  assert.equal(existsSync(stage), true);
  assert.equal(existsSync(backup), true);
});

winTest('recovery rejects staging and backup reparse points before cleanup', (t) => {
  const cases = [
    { name: 'staging-junction', field: 'stage', type: 'junction' },
    { name: 'backup-symlink', field: 'backup', type: 'dir' },
  ];

  for (const scenario of cases) {
    const root = mkdtempSync(join(tmpdir(), `cliphub-build-recovery-${scenario.name}-`));
    const bin = join(root, 'bin');
    const stage = join(bin, '.build-recovery1');
    const backup = join(bin, '.backup-recovery1');
    const journal = join(bin, '.build-publication.json');
    const target = join(bin, 'a.exe');
    const external = join(root, 'external');
    const externalArtifact = join(external, 'a.exe');
    const externalMarker = join(external, 'keep.bin');
    const artifactBytes = Buffer.from([0x01, 0x02, 0x00, 0xfe, 0xff]);
    const markerBytes = Buffer.from([0xaa, 0x00, 0xbb, 0x7f, 0x80]);
    t.after(() => rmSync(root, { recursive: true, force: true }));
    writeSet(bin, { a: 'new-a' });
    mkdirSync(external, { recursive: true });
    writeFileSync(externalArtifact, artifactBytes);
    writeFileSync(externalMarker, markerBytes);

    const linkedPath = scenario.field === 'stage' ? stage : backup;
    const regularPath = scenario.field === 'stage' ? backup : stage;
    mkdirSync(regularPath, { recursive: true });
    if (!createDirectoryReparsePoint(t, external, linkedPath, scenario.type)) {
      return;
    }
    writeFileSync(journal, JSON.stringify({
      SchemaVersion: 2,
      Phase: 'committed',
      BackupDirectory: backup,
      StagingDirectory: stage,
      Items: [{
        Name: 'a',
        Target: target,
        Backup: join(backup, 'a.exe'),
        HadOriginal: true,
        Phase: 'published',
      }],
    }));

    const result = invokeRecovery(bin);

    assert.notEqual(result.status, 0, scenario.name);
    assert.match(result.stderr, /unsafe build publication transaction directory/);
    assert.equal(readFileSync(target, 'utf8'), 'new-a');
    assert.deepEqual(readFileSync(externalArtifact), artifactBytes);
    assert.deepEqual(readFileSync(externalMarker), markerBytes);
    assert.equal(lstatSync(linkedPath).isSymbolicLink(), true);
    assert.equal(existsSync(journal), true);
  }
});

winTest('recovery accepts a present original only while its durable item phase is pending', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-pending-original-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a71');
  const backup = join(bin, '.backup-b71');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a' });
  mkdirSync(stage, { recursive: true });
  mkdirSync(backup, { recursive: true });
  writeFileSync(journal, JSON.stringify({
    SchemaVersion: 2,
    Phase: 'publishing',
    BackupDirectory: backup,
    StagingDirectory: stage,
    Items: [{
      Name: 'a',
      Target: join(bin, 'a.exe'),
      Backup: join(backup, 'a.exe'),
      HadOriginal: true,
      Phase: 'pending',
    }],
  }));

  const result = invokeRecovery(bin);

  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(stage), false);
  assert.equal(existsSync(backup), false);
});

winTest('publication classifies targets after recovering an original from a pending transaction', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-recovered-presence-'));
  const bin = join(root, 'bin');
  const oldStage = join(bin, '.build-old1');
  const oldBackup = join(bin, '.backup-old1');
  const stage = join(bin, '.build-new1');
  const backup = join(bin, '.backup-new1');
  const journal = join(bin, '.build-publication.json');
  const target = join(bin, 'a.exe');
  const classificationMarker = join(root, 'had-original.txt');
  const removalMarker = join(root, 'target-removal.txt');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(oldBackup, { a: 'old-a' });
  mkdirSync(oldStage, { recursive: true });
  writeSet(stage, { a: 'new-a' });
  writeFileSync(journal, JSON.stringify({
    SchemaVersion: 2,
    Phase: 'publishing',
    BackupDirectory: oldBackup,
    StagingDirectory: oldStage,
    Items: [{
      Name: 'a',
      Target: target,
      Backup: join(oldBackup, 'a.exe'),
      HadOriginal: true,
      Phase: 'pending',
    }],
  }));

  const result = invokePublication({
    bin,
    stage,
    backup,
    names: ['a'],
    injectedMove: `
if ($From -eq '${join(stage, 'a.exe').replaceAll("'", "''")}') {
  $document = Get-Content -LiteralPath '${journal.replaceAll("'", "''")}' -Raw | ConvertFrom-Json
  [System.IO.File]::WriteAllText(
    '${classificationMarker.replaceAll("'", "''")}',
    [string][bool]$document.Items[0].HadOriginal
  )
  throw "injected post-recovery publish failure"
}
`,
    injectedRemove: `
if ($Path -eq '${target.replaceAll("'", "''")}') {
  [System.IO.File]::WriteAllText('${removalMarker.replaceAll("'", "''")}', 'called')
}
`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /injected post-recovery publish failure/);
  assert.equal(readFileSync(classificationMarker, 'utf8'), 'True');
  assert.equal(readFileSync(target, 'utf8'), 'old-a');
  assert.equal(existsSync(removalMarker), false);
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(backup), false);
});

winTest('legacy journal cannot infer an original from a missing backup', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-legacy-missing-backup-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a72');
  const backup = join(bin, '.backup-b72');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'ambiguous-a' });
  mkdirSync(stage, { recursive: true });
  mkdirSync(backup, { recursive: true });
  writeFileSync(journal, JSON.stringify({
    SchemaVersion: 1,
    BackupDirectory: backup,
    StagingDirectory: stage,
    Items: [{
      Name: 'a',
      Target: join(bin, 'a.exe'),
      Backup: join(backup, 'a.exe'),
      HadOriginal: true,
    }],
  }));

  const result = invokeRecovery(bin);

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /original backup is missing\s+after phase 'unknown'/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'ambiguous-a');
  assert.equal(existsSync(journal), true);
});

winTest('legacy recovery migrates to restart-safe phases before deleting backups', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-legacy-cleanup-kill-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a74');
  const backup = join(bin, '.backup-b74');
  const journal = join(bin, '.build-publication.json');
  const target = join(bin, 'a.exe');
  const backupArtifact = join(backup, 'a.exe');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'candidate-a' });
  writeSet(stage, { a: 'staged-a' });
  writeSet(backup, { a: 'original-a' });
  writeFileSync(journal, JSON.stringify({
    SchemaVersion: 1,
    BackupDirectory: backup,
    StagingDirectory: stage,
    Items: [{
      Name: 'a',
      Target: target,
      Backup: backupArtifact,
      HadOriginal: true,
    }],
  }));

  const interrupted = invokeRecovery(bin, {
    injectedRemoveDirectory: `
if ($Path -eq '${backup.replaceAll("'", "''")}') {
  Remove-Item -LiteralPath $Path -Recurse -Force
  Stop-Process -Id $PID -Force
}
`,
  });

  assert.notEqual(interrupted.status, 0);
  assert.equal(readFileSync(target, 'utf8'), 'original-a');
  assert.equal(existsSync(backup), false);
  const migrated = JSON.parse(readFileSync(journal, 'utf8'));
  assert.equal(migrated.SchemaVersion, 2);
  assert.equal(migrated.Phase, 'publishing');
  assert.equal(migrated.Items[0].Phase, 'restored');

  const recovered = invokeRecovery(bin);

  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(readFileSync(target, 'utf8'), 'original-a');
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(stage), false);
});

winTest('recovery retains a no-original candidate when injected removal is a no-op', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-candidate-remove-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a73');
  const backup = join(bin, '.backup-b73');
  const journal = join(bin, '.build-publication.json');
  const target = join(bin, 'a.exe');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'candidate-a' });
  mkdirSync(stage, { recursive: true });
  mkdirSync(backup, { recursive: true });
  writeFileSync(journal, JSON.stringify({
    SchemaVersion: 2,
    Phase: 'publishing',
    BackupDirectory: backup,
    StagingDirectory: stage,
    Items: [{
      Name: 'a',
      Target: target,
      Backup: join(backup, 'a.exe'),
      HadOriginal: false,
      Phase: 'published',
    }],
  }));

  const result = invokeRecovery(bin, {
    injectedRemove: `if ($Path -eq '${target.replaceAll("'", "''")}') { return }`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /candidate removal did not complete/);
  assert.equal(readFileSync(target, 'utf8'), 'candidate-a');
  assert.equal(existsSync(journal), true);
});

winTest('recovery rejects malformed item schemas before touching transaction artifacts', (t) => {
  const malformedCases = [
    {
      name: 'missing-had-original',
      mutate(item) {
        const { HadOriginal: _ignored, ...withoutHadOriginal } = item;
        return [withoutHadOriginal];
      },
      expected: /HadOriginal must be a boolean/,
    },
    {
      name: 'string-had-original',
      mutate(item) {
        return [{ ...item, HadOriginal: 'false' }];
      },
      expected: /HadOriginal must be a boolean/,
    },
    {
      name: 'duplicate-names',
      mutate(item) {
        return [
          item,
          {
            ...item,
            Name: 'A',
            Target: item.Target.replace(/a\.exe$/i, 'A.exe'),
            Backup: item.Backup.replace(/a\.exe$/i, 'A.exe'),
          },
        ];
      },
      expected: /duplicate artifact names/,
    },
    {
      name: 'empty-items',
      mutate() {
        return [];
      },
      expected: /items must be a non-empty collection/,
    },
    {
      name: 'invalid-item-phase',
      mutate(item) {
        return [{ ...item, Phase: 'moving_somewhere' }];
      },
      expected: /invalid artifact phase/,
    },
    {
      name: 'invalid-item-path',
      mutate(item) {
        return [{ ...item, Target: join(dirname(item.Target), '..', 'outside.exe') }];
      },
      expected: /invalid artifact paths/,
    },
  ];

  for (const malformed of malformedCases) {
    const root = mkdtempSync(join(tmpdir(), `cliphub-build-${malformed.name}-`));
    t.after(() => rmSync(root, { recursive: true, force: true }));
    const bin = join(root, 'bin');
    const stage = join(bin, '.build-a91');
    const backup = join(bin, '.backup-b91');
    const journal = join(bin, '.build-publication.json');
    const target = join(bin, 'a.exe');
    const backupArtifact = join(backup, 'a.exe');
    const stagedArtifact = join(stage, 'a.exe');
    writeSet(bin, { a: 'target-a' });
    writeSet(stage, { a: 'stage-a' });
    writeSet(backup, { a: 'backup-a' });
    const item = {
      Name: 'a',
      Target: target,
      Backup: backupArtifact,
      HadOriginal: true,
      Phase: 'published',
    };
    writeFileSync(journal, JSON.stringify({
      SchemaVersion: 2,
      Phase: 'publishing',
      BackupDirectory: backup,
      StagingDirectory: stage,
      Items: malformed.mutate(item),
    }));

    const result = invokeRecovery(bin);

    assert.notEqual(result.status, 0, malformed.name);
    assert.match(result.stderr, malformed.expected);
    assert.equal(readFileSync(target, 'utf8'), 'target-a');
    assert.equal(readFileSync(backupArtifact, 'utf8'), 'backup-a');
    assert.equal(readFileSync(stagedArtifact, 'utf8'), 'stage-a');
    assert.equal(existsSync(journal), true);
  }
});

test('build entrypoint delegates publication and never removes the recovery backup', () => {
  const buildScript = readFileSync(join(repoRoot, 'scripts', 'build.ps1'), 'utf8');

  assert.match(buildScript, /build-publication\.ps1/);
  assert.match(buildScript, /Publish-BuildArtifacts/);
  assert.match(buildScript, /RequireFaceitEmbed/);
  assert.match(buildScript, /Assert-FaceitKeyEmbedded/);
  assert.match(buildScript, /-X main\.embeddedFaceitAPIKey=/);
  assert.match(buildScript, /Enter-BuildPublicationLock/);
  assert.match(buildScript, /install-go-windows\.ps1/);
  assert.match(buildScript, /Install-PinnedWindowsGo/);
  assert.match(buildScript, /Assert-GoToolchainMatchesModule/);
  assert.ok(
    buildScript.indexOf('Install-PinnedWindowsGo') < buildScript.indexOf('& go build'),
    'Go 1.26.6 must be installed before the first compiler invocation',
  );
  assert.ok(
    buildScript.indexOf('Assert-GoToolchainMatchesModule') < buildScript.indexOf('& go build'),
    'Go 1.26.6+ must be verified before the first compiler invocation',
  );
  assert.ok(
    buildScript.indexOf('Recover-BuildPublication') < buildScript.indexOf('& go build'),
    'recovery must run before the first compiler invocation',
  );
  assert.match(buildScript, /-PublicationLock \$publicationLock/);
  assert.doesNotMatch(buildScript, /Remove-Item[^\r\n]*\$backupDir/i);
});

winTest('build artifact publication recovers a process-killed mixed binary set', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-interrupted-'));
  const bin = join(root, 'bin');
  const firstStage = join(bin, '.build-a11');
  const firstBackup = join(bin, '.backup-b11');
  const retryStage = join(bin, '.build-c22');
  const retryBackup = join(bin, '.backup-d22');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(firstStage, { a: 'mixed-a', b: 'mixed-b' });

  const killAfterFirstPublish = join(firstStage, 'a.exe').replaceAll("'", "''");
  const interrupted = invokePublication({
    bin,
    stage: firstStage,
    backup: firstBackup,
    injectedMove: `
if ($From -eq '${killAfterFirstPublish}') {
  [System.IO.File]::Move($From, $To)
  Stop-Process -Id $PID -Force
}
`,
  });
  assert.notEqual(interrupted.status, 0);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'mixed-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(join(bin, '.build-publication.json')), true);

  const directRecovery = invokeRecovery(bin);
  assert.equal(directRecovery.status, 0, directRecovery.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
  assert.equal(existsSync(firstBackup), false);
  assert.equal(existsSync(firstStage), false);

  writeSet(retryStage, { a: 'new-a', b: 'new-b' });
  const recovered = invokePublication({
    bin,
    stage: retryStage,
    backup: retryBackup,
  });

  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
});

winTest('recovery is idempotent when killed after moving a backup to its target', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-recovery-restore-kill-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a80');
  const backup = join(bin, '.backup-b80');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'mixed-a', b: 'mixed-b' });

  const interruptedPublish = invokePublication({
    bin,
    stage,
    backup,
    injectedMove: `
if ($From -eq '${join(stage, 'a.exe').replaceAll("'", "''")}') {
  [System.IO.File]::Move($From, $To)
  Stop-Process -Id $PID -Force
}
`,
  });
  assert.notEqual(interruptedPublish.status, 0);

  const interruptedRecovery = invokeRecovery(bin, {
    injectedMove: `
if ($From -eq '${join(backup, 'a.exe').replaceAll("'", "''")}') {
  [System.IO.File]::Move($From, $To)
  Stop-Process -Id $PID -Force
}
`,
  });
  assert.notEqual(interruptedRecovery.status, 0);
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).Items[0].Phase, 'restoring');
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(existsSync(join(backup, 'a.exe')), false);

  const recovered = invokeRecovery(bin);

  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(backup), false);
});

winTest('live rollback is idempotent when killed after restoring one target', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-live-restore-kill-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a81');
  const backup = join(bin, '.backup-b81');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const interrupted = invokePublication({
    bin,
    stage,
    backup,
    injectedMove: `
if ($From -eq '${join(stage, 'b.exe').replaceAll("'", "''")}') {
  throw "injected publish failure"
}
if ($From -eq '${join(backup, 'b.exe').replaceAll("'", "''")}') {
  [System.IO.File]::Move($From, $To)
  Stop-Process -Id $PID -Force
}
`,
  });
  assert.notEqual(interrupted.status, 0);
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).Items[1].Phase, 'restoring');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');

  const recovered = invokeRecovery(bin);

  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'old-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'old-b');
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(backup), false);
});

winTest('committed journal recovery keeps the complete new artifact generation', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-committed-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a33');
  const backup = join(bin, '.backup-b33');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const interrupted = invokePublication({
    bin,
    stage,
    backup,
    injectedRemove: `
if ($Path -eq '${journal.replaceAll("'", "''")}') {
  Stop-Process -Id $PID -Force
}
`,
  });
  assert.notEqual(interrupted.status, 0);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  const durableJournal = JSON.parse(readFileSync(journal, 'utf8'));
  assert.equal(durableJournal.SchemaVersion, 2);
  assert.equal(durableJournal.Phase, 'committed');

  const recovered = invokeRecovery(bin);
  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(stage), false);
  assert.equal(existsSync(backup), false);
});

winTest('publication does not report success while a committed journal remains', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-journal-cleanup-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a34');
  const backup = join(bin, '.backup-b34');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const result = invokePublication({
    bin,
    stage,
    backup,
    injectedRemove: `
if ($Path -eq '${journal.replaceAll("'", "''")}') {
  return
}
`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /journal cleanup did not complete/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).Phase, 'committed');
  assert.equal(existsSync(backup), false);

  const recovered = invokeRecovery(bin);
  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(backup), false);
});

winTest('publishing retains the committed journal when directory cleanup does not complete', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-directory-cleanup-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a35');
  const backup = join(bin, '.backup-b35');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const result = invokePublication({
    bin,
    stage,
    backup,
    injectedRemoveDirectory: `
if ($Path -eq '${backup.replaceAll("'", "''")}') {
  return
}
`,
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /directory cleanup did not complete/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).Phase, 'committed');
  assert.equal(existsSync(backup), true);

  const recovered = invokeRecovery(bin);
  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(stage), false);
  assert.equal(existsSync(backup), false);
});

winTest('recovery retains a committed journal when directory cleanup must be retried', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-recovery-cleanup-'));
  const bin = join(root, 'bin');
  const stage = join(bin, '.build-a36');
  const backup = join(bin, '.backup-b36');
  const journal = join(bin, '.build-publication.json');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(stage, { a: 'new-a', b: 'new-b' });

  const interrupted = invokePublication({
    bin,
    stage,
    backup,
    injectedRemoveDirectory: `
if ($Path -eq '${backup.replaceAll("'", "''")}') {
  Stop-Process -Id $PID -Force
}
`,
  });
  assert.notEqual(interrupted.status, 0);
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).Phase, 'committed');
  assert.equal(existsSync(backup), true);

  const failedRecovery = invokeRecovery(bin, {
    injectedRemoveDirectory: `
if ($Path -eq '${backup.replaceAll("'", "''")}') {
  return
}
`,
  });
  assert.notEqual(failedRecovery.status, 0);
  assert.match(failedRecovery.stderr, /directory cleanup did not complete/);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'new-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'new-b');
  assert.equal(existsSync(journal), true);
  assert.equal(existsSync(backup), true);

  const recovered = invokeRecovery(bin);
  assert.equal(recovered.status, 0, recovered.stderr);
  assert.equal(existsSync(journal), false);
  assert.equal(existsSync(stage), false);
  assert.equal(existsSync(backup), false);
});

winTest('overlapping build publishers cannot recover a live transaction', async (t) => {
  const root = mkdtempSync(join(tmpdir(), 'cliphub-build-overlap-'));
  const bin = join(root, 'bin');
  const firstStage = join(bin, '.build-a44');
  const firstBackup = join(bin, '.backup-b44');
  const secondStage = join(bin, '.build-c55');
  const secondBackup = join(bin, '.backup-d55');
  const ready = join(root, 'first-publisher-ready');
  const release = join(root, 'release-first-publisher');
  t.after(() => rmSync(root, { recursive: true, force: true }));
  writeSet(bin, { a: 'old-a', b: 'old-b' });
  writeSet(firstStage, { a: 'first-a', b: 'first-b' });
  writeSet(secondStage, { a: 'second-a', b: 'second-b' });

  const running = startPublication({
    bin,
    stage: firstStage,
    backup: firstBackup,
    injectedMove: `
if ($From -eq '${join(firstStage, 'a.exe').replaceAll("'", "''")}') {
  [System.IO.File]::WriteAllText('${ready.replaceAll("'", "''")}', 'ready')
  while (-not (Test-Path -LiteralPath '${release.replaceAll("'", "''")}')) {
    Start-Sleep -Milliseconds 20
  }
}
`,
  });

  try {
    await waitForPath(ready);
    const overlapping = invokePublication({
      bin,
      stage: secondStage,
      backup: secondBackup,
    });
    assert.notEqual(overlapping.status, 0);
    assert.match(overlapping.stderr, /another build publication is already active/);
  } finally {
    writeFileSync(release, 'release');
  }

  const first = await running.completion;
  assert.equal(first.status, 0, first.stderr);
  assert.equal(readFileSync(join(bin, 'a.exe'), 'utf8'), 'first-a');
  assert.equal(readFileSync(join(bin, 'b.exe'), 'utf8'), 'first-b');
  assert.equal(existsSync(join(bin, '.build-publication.json')), false);
});
