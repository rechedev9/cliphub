import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { EventEmitter } from 'node:events';
import {
  existsSync,
  linkSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  renameSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import test from 'node:test';
import {
  acquirePublicationLock,
  commitPublishedDirectory,
  publicationBackupDirectory,
  publicationJournalPath,
  publicationLockPath,
  publishDirectoryAtomically,
  renameWindowsPathDurably,
  recoverInterruptedPublication,
  rollbackPublishedDirectory,
} from './atomic-publication.mjs';

test('invokes the Windows write-through rename helper without shell interpolation', () => {
  if (process.platform !== 'win32') return;
  let invocation;
  renameWindowsPathDurably('C:\\release parent\\from', 'C:\\release parent\\to', {
    spawnProcess(executable, args, options) {
      invocation = { executable, args, options };
      return { status: 0, stderr: '' };
    },
  });

  assert.equal(invocation.executable, 'powershell.exe');
  assert.equal(invocation.options.windowsHide, true);
  assert.ok(invocation.args.includes('-File'));
  assert.ok(invocation.args.some((argument) => argument.endsWith('move-publication-path.ps1')));
  assert.ok(invocation.args.includes(resolve('C:\\release parent\\from')));
  assert.ok(invocation.args.includes(resolve('C:\\release parent\\to')));
});

test('renames a real Windows directory through the write-through helper', (t) => {
  if (process.platform !== 'win32') {
    t.skip('durable rename helper is Windows-specific');
    return;
  }
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-durable-rename-'));
  const source = join(testRoot, 'source');
  const destination = join(testRoot, 'destination');
  mkdirSync(source);
  writeFileSync(join(source, 'marker.txt'), 'moved durably');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  renameWindowsPathDurably(source, destination);

  assert.equal(existsSync(source), false);
  assert.equal(readFileSync(join(destination, 'marker.txt'), 'utf8'), 'moved durably');
});

test('publishes a verified directory while replacing the previous set', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publish-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'new');
  assert.equal(existsSync(staging), false);
  assert.equal(readFileSync(join(publicationBackupDirectory(target), 'marker.txt'), 'utf8'), 'old');
  assert.equal(existsSync(publicationJournalPath(target)), true);
  assert.equal(
    JSON.parse(readFileSync(publicationJournalPath(target), 'utf8')).phase,
    'published',
  );
  commitPublishedDirectory(target);
  assert.equal(existsSync(publicationJournalPath(target)), false);
});

test('flushes the parent after journal and generation renames', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publish-flush-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const flushed = [];
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target, {
    flushDirectory(directory) {
      flushed.push(resolve(directory));
    },
  });
  commitPublishedDirectory(target, {
    flushDirectory(directory) {
      flushed.push(resolve(directory));
    },
  });

  assert.ok(flushed.length >= 6);
  assert.deepEqual(new Set(flushed), new Set([resolve(testRoot)]));
});

test('restores the previous directory when the staging rename fails', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-rollback-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  let calls = 0;
  assert.throws(
    () => publishDirectoryAtomically(staging, target, {
      rename(from, to) {
        calls += 1;
        if (calls === 2) throw new Error('injected publish failure');
        renameSync(from, to);
      },
    }),
    /injected publish failure/,
  );
  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'old');
  assert.equal(readFileSync(join(staging, 'marker.txt'), 'utf8'), 'new');
});

test('recovers an interrupted target-to-backup rename before publishing', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-recover-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  mkdirSync(backup, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(backup, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'new');
  assert.equal(readFileSync(join(backup, 'marker.txt'), 'utf8'), 'old');
});

test('rolls a published directory back after post-publication verification fails', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-post-verify-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'bad-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  rollbackPublishedDirectory(target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'old');
  assert.equal(existsSync(publicationBackupDirectory(target)), false);
  assert.equal(existsSync(publicationJournalPath(target)), false);
});

test('failed first publication is discarded durably and a retry can commit', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-first-publish-rollback-'));
  const target = join(testRoot, 'dist-installer');
  const firstStage = join(testRoot, 'stage-1');
  const retryStage = join(testRoot, 'stage-2');
  const journal = publicationJournalPath(target);
  const flushed = [];
  mkdirSync(firstStage, { recursive: true });
  writeFileSync(join(firstStage, 'marker.txt'), 'unverified-first');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(firstStage, target);
  rollbackPublishedDirectory(target, {
    flushDirectory(path) {
      flushed.push(path);
    },
  });

  assert.equal(existsSync(target), false);
  assert.equal(existsSync(journal), false);
  assert.ok(flushed.includes(testRoot));

  mkdirSync(retryStage, { recursive: true });
  writeFileSync(join(retryStage, 'marker.txt'), 'verified-retry');
  publishDirectoryAtomically(retryStage, target);
  commitPublishedDirectory(target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-retry');
  assert.equal(existsSync(journal), false);
});

test('recovery completes an interrupted first-publication discard', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-first-publish-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const journal = publicationJournalPath(target);
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(staging, 'marker.txt'), 'unverified-first');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  assert.throws(
    () => rollbackPublishedDirectory(target, {
      remove(path, options) {
        rmSync(path, options);
        if (path === target) throw new Error('injected interruption after first target removal');
      },
    }),
    /injected interruption after first target removal/,
  );
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).phase, 'restoring');
  assert.equal(existsSync(target), false);

  recoverInterruptedPublication(target);

  assert.equal(existsSync(target), false);
  assert.equal(existsSync(journal), false);
});

test('recovery retries first-publication journal cleanup from restored phase', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-first-publish-journal-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const journal = publicationJournalPath(target);
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(staging, 'marker.txt'), 'unverified-first');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  assert.throws(
    () => rollbackPublishedDirectory(target, {
      remove(path, options) {
        if (path === journal) throw new Error('injected first journal cleanup failure');
        rmSync(path, options);
      },
    }),
    /injected first journal cleanup failure/,
  );
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).phase, 'restored');
  assert.equal(existsSync(target), false);

  recoverInterruptedPublication(target);

  assert.equal(existsSync(journal), false);
});

test('an interrupted unverified target is rolled back before the next publication', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-interrupted-verify-'));
  const target = join(testRoot, 'dist-installer');
  const firstStage = join(testRoot, 'stage-1');
  const secondStage = join(testRoot, 'stage-2');
  mkdirSync(target, { recursive: true });
  mkdirSync(firstStage, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(firstStage, 'marker.txt'), 'unverified-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(firstStage, target);
  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'unverified-new');
  assert.equal(existsSync(publicationJournalPath(target)), true);

  mkdirSync(secondStage, { recursive: true });
  writeFileSync(join(secondStage, 'marker.txt'), 'verified-next');
  publishDirectoryAtomically(secondStage, target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-next');
  assert.equal(
    readFileSync(join(publicationBackupDirectory(target), 'marker.txt'), 'utf8'),
    'verified-old',
  );
  assert.equal(existsSync(publicationJournalPath(target)), true);
  commitPublishedDirectory(target);
});

test('committed recovery keeps the verified target after interrupted journal cleanup', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-committed-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  const journal = publicationJournalPath(target);
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'verified-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  assert.throws(
    () => commitPublishedDirectory(target, {
      remove(path, options) {
        if (path === journal) throw new Error('injected journal cleanup interruption');
        rmSync(path, options);
      },
    }),
    /injected journal cleanup interruption/,
  );
  assert.equal(JSON.parse(readFileSync(journal, 'utf8')).phase, 'committed');

  recoverInterruptedPublication(target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-new');
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(journal), false);
});

test('rollback preserves the target and journal when the original backup is missing', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-missing-release-backup-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  const journal = publicationJournalPath(target);
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'old');
  writeFileSync(join(staging, 'marker.txt'), 'new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  rmSync(backup, { recursive: true, force: true });

  assert.throws(
    () => rollbackPublishedDirectory(target),
    /publication backup is missing; target and journal were retained/,
  );
  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'new');
  assert.equal(existsSync(journal), true);
});

test('backup creation failure immediately recovers the prepared target', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-prepared-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const journal = publicationJournalPath(target);
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  assert.throws(
    () => publishDirectoryAtomically(staging, target, {
      rename() {
        throw new Error('injected backup interruption');
      },
    }),
    /injected backup interruption/,
  );

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-old');
  assert.equal(existsSync(journal), false);
});

test('first publication discards an unverified target when its staging rename outlives the publisher', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-first-publish-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const journal = publicationJournalPath(target);
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(staging, 'marker.txt'), 'verified-first');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  assert.throws(
    () => publishDirectoryAtomically(staging, target, {
      rename(from, to) {
        renameSync(from, to);
        if (from === staging && to === target) {
          throw new Error('injected crash after first staging rename');
        }
      },
    }),
    /injected crash after first staging rename/,
  );
  assert.equal(existsSync(target), false);
  assert.equal(existsSync(journal), false);
});

test('recovery accepts a valid Windows journal whose paths use different casing', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-path-case-recovery-'));
  const target = join(testRoot, 'Dist-Installer');
  const staging = join(testRoot, 'Stage');
  const journal = publicationJournalPath(target);
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'candidate-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  const document = JSON.parse(readFileSync(journal, 'utf8'));
  document.target_directory = document.target_directory.toUpperCase();
  document.backup_directory = document.backup_directory.toUpperCase();
  writeFileSync(journal, `${JSON.stringify(document)}\n`);

  recoverInterruptedPublication(target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-old');
  assert.equal(existsSync(journal), false);
});

test('rename completion followed by metadata flush failure immediately restores target', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-backup-flush-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  const journal = publicationJournalPath(target);
  let backupRenamed = false;
  let injected = false;
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'candidate-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  assert.throws(
    () => publishDirectoryAtomically(staging, target, {
      flushDirectory() {
        if (backupRenamed && !injected) {
          injected = true;
          throw new Error('injected flush after backup rename');
        }
      },
      rename(from, to) {
        renameSync(from, to);
        if (from === target && to === backup) backupRenamed = true;
      },
    }),
    /injected flush after backup rename/,
  );

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-old');
  assert.equal(readFileSync(join(staging, 'marker.txt'), 'utf8'), 'candidate-new');
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(journal), false);
});

test('published transition failure after staging rename immediately restores target', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-published-transition-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  const journal = publicationJournalPath(target);
  let stagingRenamed = false;
  let postStagingFlushes = 0;
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'candidate-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  assert.throws(
    () => publishDirectoryAtomically(staging, target, {
      flushDirectory() {
        if (!stagingRenamed) return;
        postStagingFlushes += 1;
        if (postStagingFlushes === 2) {
          throw new Error('injected published transition flush failure');
        }
      },
      rename(from, to) {
        renameSync(from, to);
        if (from === staging && to === target) stagingRenamed = true;
      },
    }),
    /injected published transition flush failure/,
  );

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-old');
  assert.equal(existsSync(staging), false);
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(journal), false);
});

test('publication preserves mutation and recovery errors', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-double-publication-failure-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  let backupRenamed = false;
  let failureCount = 0;
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'candidate-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  let error;
  try {
    publishDirectoryAtomically(staging, target, {
      flushDirectory() {
        if (!backupRenamed) return;
        failureCount += 1;
        throw new Error(`injected ordered flush failure ${failureCount}`);
      },
      rename(from, to) {
        renameSync(from, to);
        if (from === target && to === backup) backupRenamed = true;
      },
    });
  } catch (caught) {
    error = caught;
  }

  assert.ok(error instanceof AggregateError);
  assert.equal(error.errors.length, 2);
  assert.match(error.errors[0].message, /ordered flush failure 1/);
  assert.match(error.errors[1].message, /ordered flush failure 2/);
});

test('live rollback accepts an injected error after its restore rename completed', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-restore-recovery-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  const journal = publicationJournalPath(target);
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'candidate-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  rollbackPublishedDirectory(target, {
    rename(from, to) {
      renameSync(from, to);
      if (from === backup && to === target) {
        throw new Error('injected interruption after restore rename');
      }
    },
  });

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-old');
  assert.equal(existsSync(backup), false);
  assert.equal(existsSync(journal), false);
});

test('recovery completes a restoring journal after its backup rename', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-restoring-journal-'));
  const target = join(testRoot, 'dist-installer');
  const staging = join(testRoot, 'stage');
  const backup = publicationBackupDirectory(target);
  const journal = publicationJournalPath(target);
  mkdirSync(target, { recursive: true });
  mkdirSync(staging, { recursive: true });
  writeFileSync(join(target, 'marker.txt'), 'verified-old');
  writeFileSync(join(staging, 'marker.txt'), 'candidate-new');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  publishDirectoryAtomically(staging, target);
  const document = JSON.parse(readFileSync(journal, 'utf8'));
  document.phase = 'restoring';
  writeFileSync(journal, `${JSON.stringify(document)}\n`);
  rmSync(target, { recursive: true });
  renameSync(backup, target);

  recoverInterruptedPublication(target);

  assert.equal(readFileSync(join(target, 'marker.txt'), 'utf8'), 'verified-old');
  assert.equal(existsSync(journal), false);
});

test('publication lock excludes another live process and releases on owner death', async (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publication-lock-'));
  const target = join(testRoot, 'dist-installer');
  const moduleURL = new URL('./atomic-publication.mjs', import.meta.url).href;
  const childCode = `
    import { acquirePublicationLock } from ${JSON.stringify(moduleURL)};
    await acquirePublicationLock(process.env.FRAGFORGE_TEST_TARGET);
    process.stdout.write('locked\\n');
    setInterval(() => {}, 1000);
  `;
  const child = spawn(process.execPath, ['--input-type=module', '--eval', childCode], {
    env: { ...process.env, FRAGFORGE_TEST_TARGET: target },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  t.after(() => {
    if (child.exitCode === null) child.kill();
    rmSync(testRoot, { recursive: true, force: true });
  });

  await waitForOutput(child, 'locked');
  await assert.rejects(
    acquirePublicationLock(target),
    /another distribution build is already running/,
  );
  child.kill();
  await waitForExit(child);

  const reclaimers = await Promise.allSettled([
    acquirePublicationLock(target),
    acquirePublicationLock(target),
  ]);
  assert.equal(reclaimers.filter(({ status }) => status === 'fulfilled').length, 1);
  assert.equal(reclaimers.filter(({ status }) => status === 'rejected').length, 1);
  await reclaimers.find(({ status }) => status === 'fulfilled').value();
  assert.equal(
    JSON.parse(readFileSync(publicationLockPath(target), 'utf8')).state,
    'released',
  );
});

test('publication lock grants exactly one of two simultaneous contenders', async (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publication-lock-race-'));
  const target = join(testRoot, 'dist-installer');
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  const contenders = await Promise.allSettled([
    acquirePublicationLock(target),
    acquirePublicationLock(target),
  ]);
  assert.equal(contenders.filter(({ status }) => status === 'fulfilled').length, 1);
  assert.equal(contenders.filter(({ status }) => status === 'rejected').length, 1);
  assert.match(
    contenders.find(({ status }) => status === 'rejected').reason.message,
    /another distribution build is already running/,
  );
  await contenders.find(({ status }) => status === 'fulfilled').value();
});

test('publication lock rejects a symbolic-link lock path without touching its target', async (t) => {
  if (process.platform !== 'win32') {
    t.skip('publication lock helper is Windows-specific');
    return;
  }

  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publication-lock-symlink-'));
  const target = join(testRoot, 'dist-installer');
  const protectedTarget = join(testRoot, 'protected.txt');
  const lockPath = publicationLockPath(target);
  const sentinel = 'protected symlink target';
  writeFileSync(protectedTarget, sentinel);
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));
  try {
    symlinkSync(protectedTarget, lockPath, 'file');
  } catch (error) {
    if (error?.code === 'EPERM' || error?.code === 'EACCES') {
      t.skip(`symbolic links are unavailable: ${error.code}`);
      return;
    }
    throw error;
  }

  await assert.rejects(
    acquirePublicationLock(target),
    /publication lock path must not be a reparse point/,
  );

  assert.equal(readFileSync(protectedTarget, 'utf8'), sentinel);
  assert.equal(readFileSync(lockPath, 'utf8'), sentinel);
});

test('publication lock rejects a hard-linked lock path without touching its target', async (t) => {
  if (process.platform !== 'win32') {
    t.skip('publication lock helper is Windows-specific');
    return;
  }

  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publication-lock-hardlink-'));
  const target = join(testRoot, 'dist-installer');
  const protectedTarget = join(testRoot, 'protected.txt');
  const lockPath = publicationLockPath(target);
  const sentinel = 'protected hardlink target';
  writeFileSync(protectedTarget, sentinel);
  linkSync(protectedTarget, lockPath);
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));

  await assert.rejects(
    acquirePublicationLock(target),
    /publication lock path must have exactly one hard link/,
  );

  assert.equal(readFileSync(protectedTarget, 'utf8'), sentinel);
  assert.equal(readFileSync(lockPath, 'utf8'), sentinel);
});

test('publication fence survives helper death while its Node owner is alive', async (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-publication-fence-'));
  const target = join(testRoot, 'dist-installer');
  let helper;
  const release = await acquirePublicationLock(target, {
    onHelperStarted(child) {
      helper = child;
    },
  });
  t.after(async () => {
    await release().catch(() => {});
    rmSync(testRoot, { recursive: true, force: true });
  });

  helper.kill();
  await waitForExit(helper);
  const heldFence = JSON.parse(readFileSync(publicationLockPath(target), 'utf8'));
  assert.equal(heldFence.state, 'held');
  assert.equal(heldFence.owner_pid, process.pid);
  assert.match(heldFence.owner_started_ticks, /^[0-9]+$/);
  await assert.rejects(
    acquirePublicationLock(target),
    /another distribution build is already running/,
  );

  await release();
  await release();
  assert.equal(
    JSON.parse(readFileSync(publicationLockPath(target), 'utf8')).state,
    'released',
  );

  const releaseNext = await acquirePublicationLock(target);
  await releaseNext();
});

test('release accepts RELEASED delivered after exit for normal and fallback helpers', async (t) => {
  await t.test('normal helper', async () => {
    const helper = fakeLockHelper((input) => {
      assert.equal(input, 'RELEASE\n');
      queueMicrotask(() => finishFakeHelper(helper, {
        code: 0,
        stdout: 'RELEASED\n',
      }));
    });
    let spawnCalls = 0;
    const releasePromise = acquirePublicationLock('C:\\fake-normal-target', {
      spawnProcess() {
        spawnCalls += 1;
        queueMicrotask(() => helper.stdout.emit('data', 'LOCKED\n'));
        return helper;
      },
    });

    const release = await releasePromise;
    await release();

    assert.equal(spawnCalls, 1);
  });

  await t.test('fallback helper', async () => {
    const original = fakeLockHelper();
    const fallback = fakeLockHelper();
    let spawnCalls = 0;
    const releasePromise = acquirePublicationLock('C:\\fake-fallback-target', {
      spawnProcess() {
        spawnCalls += 1;
        if (spawnCalls === 1) {
          queueMicrotask(() => original.stdout.emit('data', 'LOCKED\n'));
          return original;
        }
        queueMicrotask(() => finishFakeHelper(fallback, {
          code: 0,
          stdout: 'RELEASED\n',
        }));
        return fallback;
      },
    });

    const release = await releasePromise;
    finishFakeHelper(original, { code: 1 });
    await release();

    assert.equal(spawnCalls, 2);
  });
});

test('missing release marker waits for stdio and preserves the helper failure', async () => {
  const failed = fakeLockHelper();
  const fallback = fakeLockHelper();
  let spawnCalls = 0;
  let settled = false;
  const acquisition = acquirePublicationLock('C:\\fake-failed-target', {
    spawnProcess() {
      spawnCalls += 1;
      if (spawnCalls === 1) return failed;
      queueMicrotask(() => finishFakeHelper(fallback, {
        code: 0,
        stdout: 'RELEASED\n',
      }));
      return fallback;
    },
  }).finally(() => {
    settled = true;
  });

  failed.stderr.emit('data', 'deterministic helper failure');
  markFakeHelperExited(failed, 7);
  await new Promise((resolveTurn) => setImmediate(resolveTurn));
  assert.equal(settled, false, 'exit alone must not declare the marker missing');

  finishFakeHelperStdio(failed);
  await assert.rejects(
    acquisition,
    /publication lock helper failed \(7\): deterministic helper failure/,
  );
  assert.equal(spawnCalls, 2);
});

test('waitForExit recognizes an already signalled child', async () => {
  await waitForExit({
    exitCode: null,
    signalCode: 'SIGTERM',
    once() {
      throw new Error('must not subscribe after a signal was recorded');
    },
  });
});

function fakeLockHelper(onInput) {
  const child = new EventEmitter();
  child.exitCode = null;
  child.signalCode = null;
  child.stdout = new EventEmitter();
  child.stderr = new EventEmitter();
  child.stdin = new EventEmitter();
  child.stdout.setEncoding = () => {};
  child.stderr.setEncoding = () => {};
  child.stdin.end = (input = '') => {
    onInput?.(input);
  };
  return child;
}

function markFakeHelperExited(child, code, signal = null) {
  child.exitCode = code;
  child.signalCode = signal;
  child.emit('exit', code, signal);
}

function finishFakeHelperStdio(child, {
  code = child.exitCode,
  signal = child.signalCode,
  stdout = '',
  stderr = '',
} = {}) {
  if (stdout !== '') child.stdout.emit('data', stdout);
  if (stderr !== '') child.stderr.emit('data', stderr);
  child.stdout.emit('end');
  child.stderr.emit('end');
  child.emit('close', code, signal);
}

function finishFakeHelper(child, options) {
  markFakeHelperExited(child, options.code, options.signal);
  finishFakeHelperStdio(child, options);
}

function waitForOutput(child, expected) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(
      () => reject(new Error(`timed out waiting for child output: ${expected}`)),
      5_000,
    );
    let output = '';
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (chunk) => {
      output += chunk;
      if (!output.includes(expected)) return;
      clearTimeout(timeout);
      resolve();
    });
    child.once('error', (error) => {
      clearTimeout(timeout);
      reject(error);
    });
    child.once('exit', (code) => {
      if (output.includes(expected)) return;
      clearTimeout(timeout);
      reject(new Error(`child exited before acquiring lock (${code})`));
    });
  });
}

function waitForExit(child) {
  if (child.exitCode !== null || child.signalCode !== null) return Promise.resolve();
  return new Promise((resolve) => child.once('exit', resolve));
}
