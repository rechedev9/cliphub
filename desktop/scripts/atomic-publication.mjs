import {
  existsSync,
  lstatSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { spawn, spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { basename, dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const JOURNAL_SCHEMA_VERSION = 2;
const JOURNAL_PHASES = new Set([
  'prepared',
  'backup_created',
  'published',
  'restoring',
  'restored',
  'committed',
]);

export function publicationBackupDirectory(targetDirectory) {
  return join(dirname(targetDirectory), `.${basename(targetDirectory)}-backup`);
}

export function publicationJournalPath(targetDirectory) {
  return join(dirname(targetDirectory), `.${basename(targetDirectory)}-publication.json`);
}

export function publicationLockPath(targetDirectory) {
  return join(dirname(targetDirectory), `.${basename(targetDirectory)}-publication.lock`);
}

/** Renames one publication path with Win32 write-through semantics. */
export function renameWindowsPathDurably(from, to, {
  powershell = 'powershell.exe',
  spawnProcess = spawnSync,
} = {}) {
  if (process.platform !== 'win32') {
    renameSync(from, to);
    return;
  }
  const helperPath = join(
    dirname(fileURLToPath(import.meta.url)),
    'move-publication-path.ps1',
  );
  const args = [
    '-NoLogo',
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-File',
    helperPath,
    '-From',
    resolve(from),
    '-To',
    resolve(to),
  ];
  if (existsSync(to)) args.push('-ReplaceExisting');
  const result = spawnProcess(powershell, args, {
    encoding: 'utf8',
    windowsHide: true,
  });
  if (result.error) {
    throw new Error(`[dist] durable Windows rename could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    const detail = (result.stderr ?? '').trim();
    throw new Error(
      `[dist] durable Windows rename failed (${result.status})${detail ? `: ${detail}` : ''}`,
    );
  }
}

const noAdditionalMetadataFlush = () => {};

/**
 * Acquires the process-wide publication lease through a Win32-exclusive handle
 * plus a persistent owner fence. If the helper dies, PID/start identity keeps
 * the live Node publisher fenced; if Node dies, the next exclusive helper can
 * prove that identity stale before replacing it.
 */
export async function acquirePublicationLock(targetDirectory, {
  onHelperStarted,
  powershell = 'powershell.exe',
  spawnProcess = spawn,
} = {}) {
  const lockPath = publicationLockPath(targetDirectory);
  const helperPath = join(dirname(fileURLToPath(import.meta.url)), 'hold-publication-lock.ps1');
  const ownerPID = process.pid;
  const token = randomUUID().replaceAll('-', '');
  const child = spawnLockHelper({
    helperPath,
    lockPath,
    mode: 'acquire',
    ownerPID,
    powershell,
    spawnProcess,
    token,
  });
  try {
    await waitForLockHelper(child, 'LOCKED');
  } catch (error) {
    if (error?.code !== 'PUBLICATION_LOCK_CONTENDED') {
      await releasePublicationFence({
        helperPath,
        lockPath,
        ownerPID,
        powershell,
        spawnProcess,
        token,
      }).catch(() => {});
    }
    throw error;
  }
  onHelperStarted?.(child);

  let releasePromise;
  return () => {
    releasePromise ??= (async () => {
      let releasedByOriginalHelper = false;
      if (child.exitCode === null && child.signalCode === null) {
        const acknowledged = waitForLockHelper(child, 'RELEASED');
        child.stdin.on('error', () => {});
        child.stdin.end('RELEASE\n');
        try {
          await acknowledged;
          releasedByOriginalHelper = true;
        } catch {
          // A dead helper is recovered below through the persistent fence.
        }
        await waitForProcessExit(child);
      }
      if (!releasedByOriginalHelper) {
        // The persistent fence is the fallback when the handle-owning helper
        // dies after LOCKED but before acknowledging RELEASED.
        await releasePublicationFence({
          helperPath,
          lockPath,
          ownerPID,
          powershell,
          spawnProcess,
          token,
        });
      }
    })();
    return releasePromise;
  };
}

/**
 * Restores the last verified directory after a process/power interruption.
 * Recovery is deliberately conservative: no target is deleted until a valid
 * rollback directory has been established.
 */
export function recoverInterruptedPublication(targetDirectory, {
  flushDirectory = noAdditionalMetadataFlush,
  rename = renameWindowsPathDurably,
  remove = rmSync,
} = {}) {
  const journalPath = publicationJournalPath(targetDirectory);
  if (!existsSync(journalPath)) return false;
  const journal = readJournal(targetDirectory);
  const backupDirectory = publicationBackupDirectory(targetDirectory);
  const targetExists = directoryExists(targetDirectory, 'publication target');
  const backupExists = directoryExists(backupDirectory, 'publication backup');

  if (journal.phase === 'committed') {
    if (targetExists) {
      if (backupExists) remove(backupDirectory, { recursive: true, force: true });
      remove(journalPath, { force: true });
      return true;
    }
    if (backupExists) {
      renameDurably(backupDirectory, targetDirectory, rename, flushDirectory);
      remove(journalPath, { force: true });
      return true;
    }
    throw new Error('[dist] committed publication has neither target nor backup; refusing recovery');
  }

  if (journal.had_target) {
    if (backupExists) {
      restorePreviousPublication(targetDirectory, journal, {
        flushDirectory,
        rename,
        remove,
      });
      return true;
    }

    if (
      (journal.phase === 'restoring' || journal.phase === 'restored')
      && targetExists
    ) {
      const restored = journal.phase === 'restored'
        ? journal
        : transitionJournal(journalPath, journal, 'restored', flushDirectory);
      if (restored.phase === 'restored') remove(journalPath, { force: true });
      return true;
    }

    if (journal.phase === 'prepared' && targetExists) {
      // The journal became durable before target -> backup. With no backup,
      // the only remaining directory is retained as the prior generation.
      remove(journalPath, { force: true });
      return true;
    }

    throw new Error('[dist] publication backup is missing; target and journal were retained');
  }

  if (backupExists) {
    throw new Error(
      '[dist] first publication unexpectedly has a rollback generation; refusing recovery',
    );
  }

  if (
    journal.phase === 'published'
    || journal.phase === 'restoring'
    || journal.phase === 'restored'
    || (journal.phase === 'backup_created' && targetExists)
  ) {
    discardFirstPublication(targetDirectory, journal, {
      flushDirectory,
      remove,
    });
    return true;
  }

  if (!targetExists && (
    journal.phase === 'prepared'
    || journal.phase === 'backup_created'
  )) {
    removeJournalDurably(journalPath, flushDirectory, remove);
    return true;
  }

  // There is no previous generation against which this first publication can
  // be rolled back. Preserve the only candidate instead of deleting it.
  throw new Error('[dist] publication has no rollback generation; target and journal were retained');
}

/**
 * Swaps a fully verified staging directory into the canonical location.
 */
export function publishDirectoryAtomically(stagingDirectory, targetDirectory, {
  flushDirectory = noAdditionalMetadataFlush,
  rename = renameWindowsPathDurably,
  remove = rmSync,
} = {}) {
  if (!directoryExists(stagingDirectory, 'verified staging directory')) {
    throw new Error('[dist] verified staging directory is missing');
  }
  const backupDirectory = publicationBackupDirectory(targetDirectory);
  const journalPath = publicationJournalPath(targetDirectory);

  recoverInterruptedPublication(targetDirectory, { flushDirectory, rename, remove });
  const targetExists = directoryExists(targetDirectory, 'publication target');
  const backupExists = directoryExists(backupDirectory, 'publication backup');
  if (backupExists) {
    if (targetExists) {
      // No journal means the target is committed. The retained older
      // generation can make room for the next transaction.
      remove(backupDirectory, { recursive: true, force: true });
    } else {
      renameDurably(backupDirectory, targetDirectory, rename, flushDirectory);
    }
  }
  const hadTarget = directoryExists(targetDirectory, 'publication target');
  let journal = {
    schema_version: JOURNAL_SCHEMA_VERSION,
    transaction_id: randomUUID(),
    phase: 'prepared',
    staging_directory: resolve(stagingDirectory),
    target_directory: resolve(targetDirectory),
    backup_directory: resolve(backupDirectory),
    had_target: hadTarget,
  };

  writeJournal(journalPath, journal, flushDirectory);
  try {
    if (hadTarget) renameDurably(targetDirectory, backupDirectory, rename, flushDirectory);
    journal = transitionJournal(journalPath, journal, 'backup_created', flushDirectory);
    renameDurably(stagingDirectory, targetDirectory, rename, flushDirectory);
    transitionJournal(journalPath, journal, 'published', flushDirectory);
  } catch (error) {
    try {
      recoverInterruptedPublication(targetDirectory, { flushDirectory, rename, remove });
    } catch (recoveryError) {
      throw new AggregateError(
        [error, recoveryError],
        '[dist] publication failed and recovery requires manual intervention',
      );
    }
    throw error;
  }
}

/** Marks the current target verified before any recovery metadata is cleaned. */
export function commitPublishedDirectory(targetDirectory, {
  flushDirectory = noAdditionalMetadataFlush,
  remove = rmSync,
} = {}) {
  const journalPath = publicationJournalPath(targetDirectory);
  const journal = readJournal(targetDirectory);
  if (journal.phase !== 'published') {
    throw new Error(`[dist] cannot commit publication in ${journal.phase} phase`);
  }
  if (!directoryExists(targetDirectory, 'publication target')) {
    throw new Error('[dist] cannot commit a missing publication target');
  }
  transitionJournal(journalPath, journal, 'committed', flushDirectory);
  remove(journalPath, { force: true });
}

export function rollbackPublishedDirectory(targetDirectory, {
  flushDirectory = noAdditionalMetadataFlush,
  rename = renameWindowsPathDurably,
  remove = rmSync,
} = {}) {
  const journalPath = publicationJournalPath(targetDirectory);
  const journal = readJournal(targetDirectory);
  const backupDirectory = publicationBackupDirectory(targetDirectory);
  const targetExists = directoryExists(targetDirectory, 'publication target');
  const backupExists = directoryExists(backupDirectory, 'publication backup');

  if (journal.phase === 'committed') {
    throw new Error('[dist] publication is already committed; refusing rollback');
  }
  if (journal.had_target) {
    if (!backupExists) {
      if (
        (journal.phase === 'restoring' || journal.phase === 'restored')
        && targetExists
      ) {
        const restored = journal.phase === 'restored'
          ? journal
          : transitionJournal(journalPath, journal, 'restored', flushDirectory);
        if (restored.phase === 'restored') remove(journalPath, { force: true });
        return;
      }
      throw new Error('[dist] publication backup is missing; target and journal were retained');
    }
    restorePreviousPublication(targetDirectory, journal, {
      flushDirectory,
      rename,
      remove,
    });
    return;
  }

  if (
    journal.phase === 'published'
    || journal.phase === 'restoring'
    || journal.phase === 'restored'
    || (journal.phase === 'backup_created' && targetExists)
  ) {
    discardFirstPublication(targetDirectory, journal, {
      flushDirectory,
      remove,
    });
    return;
  }
  if (!targetExists && (
    journal.phase === 'prepared'
    || journal.phase === 'backup_created'
  )) {
    removeJournalDurably(journalPath, flushDirectory, remove);
    return;
  }
  throw new Error('[dist] publication has no rollback generation; target and journal were retained');
}

function discardFirstPublication(targetDirectory, journal, {
  flushDirectory,
  remove,
}) {
  const journalPath = publicationJournalPath(targetDirectory);
  const backupDirectory = publicationBackupDirectory(targetDirectory);
  if (directoryExists(backupDirectory, 'publication backup')) {
    throw new Error(
      '[dist] first publication unexpectedly has a rollback generation; refusing recovery',
    );
  }
  const targetExists = directoryExists(targetDirectory, 'publication target');
  if (
    targetExists
    && directoryExists(journal.staging_directory, 'publication staging directory')
  ) {
    throw new Error(
      '[dist] first publication has both staging and target directories; refusing recovery',
    );
  }

  let restoring = journal;
  if (journal.phase !== 'restoring' && journal.phase !== 'restored') {
    restoring = transitionJournal(journalPath, journal, 'restoring', flushDirectory);
  }
  if (targetExists) {
    remove(targetDirectory, { recursive: true, force: true });
    if (directoryExists(targetDirectory, 'publication target')) {
      throw new Error('[dist] unverified first publication target removal did not complete');
    }
    flushDirectory(dirname(resolve(targetDirectory)));
  }
  if (restoring.phase !== 'restored') {
    transitionJournal(journalPath, restoring, 'restored', flushDirectory);
  }
  removeJournalDurably(journalPath, flushDirectory, remove);
}

function restorePreviousPublication(targetDirectory, journal, {
  flushDirectory,
  rename,
  remove,
}) {
  const journalPath = publicationJournalPath(targetDirectory);
  const backupDirectory = publicationBackupDirectory(targetDirectory);
  const restoring = journal.phase === 'restoring'
    ? journal
    : transitionJournal(journalPath, journal, 'restoring', flushDirectory);
  if (directoryExists(targetDirectory, 'publication target')) {
    remove(targetDirectory, { recursive: true, force: true });
  }
  try {
    renameDurably(backupDirectory, targetDirectory, rename, flushDirectory);
  } catch (error) {
    const targetExists = directoryExists(targetDirectory, 'publication target');
    const backupExists = directoryExists(backupDirectory, 'publication backup');
    if (!targetExists || backupExists) throw error;
  }
  transitionJournal(journalPath, restoring, 'restored', flushDirectory);
  remove(journalPath, { force: true });
}

function removeJournalDurably(journalPath, flushDirectory, remove) {
  remove(journalPath, { force: true });
  if (existsSync(journalPath)) {
    throw new Error('[dist] publication journal cleanup did not complete');
  }
  flushDirectory(dirname(resolve(journalPath)));
}

function transitionJournal(journalPath, journal, phase, flushDirectory) {
  const next = { ...journal, phase };
  writeJournal(journalPath, next, flushDirectory);
  return next;
}

function writeJournal(journalPath, document, flushDirectory) {
  const temporary = `${journalPath}.tmp-${randomUUID()}`;
  try {
    writeFileSync(temporary, `${JSON.stringify(document)}\n`, {
      encoding: 'utf8',
      flag: 'wx',
      flush: true,
    });
    renameDurably(temporary, journalPath, renameWindowsPathDurably, flushDirectory);
  } finally {
    rmSync(temporary, { force: true });
  }
}

function renameDurably(from, to, rename, flushDirectory) {
  rename(from, to);
  const parentDirectories = new Map();
  for (const path of [from, to]) {
    const parent = dirname(resolve(path));
    parentDirectories.set(parent.toLocaleLowerCase('en-US'), parent);
  }
  for (const parent of parentDirectories.values()) flushDirectory(parent);
}

function readJournal(targetDirectory) {
  const journalPath = publicationJournalPath(targetDirectory);
  let journal;
  try {
    journal = JSON.parse(readFileSync(journalPath, 'utf8'));
  } catch {
    throw new Error('[dist] publication journal is unreadable; refusing destructive recovery');
  }

  const expectedTarget = resolve(targetDirectory);
  const expectedBackup = resolve(publicationBackupDirectory(targetDirectory));
  if (
    journal?.schema_version !== JOURNAL_SCHEMA_VERSION
    || typeof journal.transaction_id !== 'string'
    || journal.transaction_id.length === 0
    || !JOURNAL_PHASES.has(journal.phase)
    || typeof journal.staging_directory !== 'string'
    || !sameWindowsPath(journal.target_directory, expectedTarget)
    || !sameWindowsPath(journal.backup_directory, expectedBackup)
    || typeof journal.had_target !== 'boolean'
  ) {
    throw new Error('[dist] publication journal is invalid; refusing destructive recovery');
  }
  return journal;
}

function sameWindowsPath(candidate, expected) {
  if (typeof candidate !== 'string') return false;
  return resolve(candidate).toLocaleLowerCase('en-US')
    === resolve(expected).toLocaleLowerCase('en-US');
}

function directoryExists(path, label) {
  if (!existsSync(path)) return false;
  const info = lstatSync(path);
  if (!info.isDirectory() || info.isSymbolicLink()) {
    throw new Error(`[dist] ${label} is not a regular directory; refusing destructive recovery`);
  }
  return true;
}

function spawnLockHelper({
  helperPath,
  lockPath,
  mode,
  ownerPID,
  powershell,
  spawnProcess,
  token,
}) {
  return spawnProcess(powershell, [
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-File',
    helperPath,
    '-Mode',
    mode,
    '-LockPath',
    lockPath,
    '-OwnerPID',
    String(ownerPID),
    '-Token',
    token,
  ], {
    stdio: ['pipe', 'pipe', 'pipe'],
    windowsHide: true,
  });
}

async function releasePublicationFence(options) {
  const child = spawnLockHelper({ ...options, mode: 'release' });
  child.stdin.end();
  await waitForLockHelper(child, 'RELEASED');
  await waitForProcessExit(child);
}

function waitForLockHelper(child, successMarker) {
  return new Promise((resolve, reject) => {
    let settled = false;
    let output = '';
    let errors = '';
    let processEnded = child.exitCode !== null || child.signalCode !== null;
    let exitCode = child.exitCode;
    let exitSignal = child.signalCode;
    let stdoutEnded = false;
    let stderrEnded = false;
    let streamsClosed = false;

    const rejectMissingMarker = () => {
      if (
        settled
        || !processEnded
        || (!streamsClosed && (!stdoutEnded || !stderrEnded))
      ) return;
      settled = true;
      if (exitCode === 2 || errors.includes('another distribution build is already running')) {
        const error = new Error('[dist] another distribution build is already running');
        error.code = 'PUBLICATION_LOCK_CONTENDED';
        reject(error);
        return;
      }
      const status = exitCode ?? exitSignal ?? 'unknown';
      reject(new Error(`[dist] publication lock helper failed (${status}): ${errors.trim()}`));
    };

    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (chunk) => {
      output += chunk;
      if (!settled && output.includes(successMarker)) {
        settled = true;
        resolve();
      }
    });
    child.stderr.on('data', (chunk) => {
      errors += chunk;
    });
    child.stdout.once('end', () => {
      stdoutEnded = true;
      rejectMissingMarker();
    });
    child.stderr.once('end', () => {
      stderrEnded = true;
      rejectMissingMarker();
    });
    child.once('error', (error) => {
      if (settled) return;
      settled = true;
      reject(error);
    });
    child.once('exit', (code, signal) => {
      processEnded = true;
      exitCode = code;
      exitSignal = signal;
      rejectMissingMarker();
    });
    child.once('close', (code, signal) => {
      processEnded = true;
      stdoutEnded = true;
      stderrEnded = true;
      streamsClosed = true;
      if (code !== null) exitCode = code;
      if (signal !== null) exitSignal = signal;
      rejectMissingMarker();
    });
  });
}

function waitForProcessExit(child) {
  if (child.exitCode !== null || child.signalCode !== null) return Promise.resolve();
  return new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', resolve);
  });
}
