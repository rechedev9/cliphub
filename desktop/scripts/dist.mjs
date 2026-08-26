import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { rmSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import {
  goRuntimeBuildEnvironment,
  releaseBuildEnvironment,
} from './build-environment.mjs';
import {
  acquirePublicationLock,
  commitPublishedDirectory,
  publishDirectoryAtomically,
  recoverInterruptedPublication,
  rollbackPublishedDirectory,
} from './atomic-publication.mjs';
import { runDistributionBuildSteps } from './distribution-build.mjs';
import { readPinnedHLAETool, verifyBundledHLAE } from './hlae-bundle.mjs';
import { releasePaths, verifyReleaseChecksums, writeReleaseChecksums } from './release-integrity.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const desktop = join(here, '..');
const repo = join(desktop, '..');
const canonicalRelease = releasePaths(desktop);
const stagingDirectory = join(
  desktop,
  `.dist-installer-stage-${randomUUID().replaceAll('-', '')}`,
);
const stagedRelease = releasePaths(desktop, stagingDirectory);

if (process.argv.length > 2) {
  console.error('[dist] unsupported build argument');
  process.exit(1);
}
const sanitizedEnvironment = releaseBuildEnvironment();
const goEnvironment = goRuntimeBuildEnvironment();

let failed = false;
let published = false;
let releasePublicationLock;
try {
  // The lease covers recovery, the complete build, canonical verification, and
  // commit. A concurrent dist must never mistake this live journal for a crash.
  releasePublicationLock = await acquirePublicationLock(canonicalRelease.directory);
  recoverInterruptedPublication(canonicalRelease.directory);
  runDistributionBuildSteps({ repo, desktop, environment: goEnvironment });
  execFileSync(process.execPath, [
    join(desktop, 'node_modules', 'electron-builder', 'cli.js'),
    '--win',
    'nsis',
    `--config.directories.output=${stagingDirectory}`,
  ], {
    cwd: desktop,
    env: sanitizedEnvironment,
    stdio: 'inherit',
  });
  const hlae = readPinnedHLAETool(desktop);
  verifyBundledHLAE(
    join(stagingDirectory, 'win-unpacked', 'resources', 'hlae', hlae.archiveName),
    hlae,
  );
  await requireNonEmptyFile(stagedRelease.artifacts[0], 'installer');
  await requireNonEmptyFile(stagedRelease.artifacts[1], 'installer blockmap');
  await writeReleaseChecksums(stagedRelease.artifacts, stagedRelease.checksum);
  await verifyReleaseChecksums(stagedRelease.artifacts, stagedRelease.checksum);
  publishDirectoryAtomically(stagingDirectory, canonicalRelease.directory);
  published = true;
  await verifyReleaseChecksums(canonicalRelease.artifacts, canonicalRelease.checksum);
  commitPublishedDirectory(canonicalRelease.directory);
} catch (err) {
  failed = true;
  if (published) {
    try {
      rollbackPublishedDirectory(canonicalRelease.directory);
    } catch (rollbackError) {
      console.error(rollbackError instanceof Error ? rollbackError.message : '[dist] rollback failed');
    }
  }
  console.error(err instanceof Error && err.message.startsWith('[dist]')
    ? err.message
    : '[dist] build or verification failed');
} finally {
  // A failed build removes only its private staging area; the last verified
  // canonical installer directory is deliberately untouched.
  if (failed) rmSync(stagingDirectory, { recursive: true, force: true });
  if (releasePublicationLock !== undefined) await releasePublicationLock();
}

if (failed) process.exit(1);

async function requireNonEmptyFile(filePath, label) {
  const info = await waitForFile(filePath, label);
  if (info.size === 0) throw new Error(`[dist] ${label} was not produced`);
}

async function waitForFile(filePath, label) {
  const deadline = Date.now() + 15_000;
  while (true) {
    try {
      const info = statSync(filePath);
      if (info.isFile()) return info;
    } catch {
      // Windows security scanning can briefly hide a newly signed NSIS output.
    }
    if (Date.now() >= deadline) throw new Error(`[dist] ${label} was not produced`);
    await delay(200);
  }
}
