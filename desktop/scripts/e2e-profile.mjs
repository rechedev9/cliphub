import {
  cpSync,
  existsSync,
  lstatSync,
  mkdtempSync,
  rmSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join, resolve } from 'node:path';

export const E2E_USER_DATA_ENV = 'CLIPHUB_E2E_USER_DATA';
export const E2E_TOOL_FIXTURE_ENV = 'CLIPHUB_E2E_TOOL_FIXTURE';

function safeLabel(label) {
  return String(label).replaceAll(/[^A-Za-z0-9_-]/g, '-').slice(0, 40) || 'run';
}

function verifiedFixtureDirectory(value) {
  if (!value) return null;
  const fixture = resolve(value);
  const info = lstatSync(fixture);
  if (!info.isDirectory() || info.isSymbolicLink()) {
    throw new Error(`${E2E_TOOL_FIXTURE_ENV} must name a real directory`);
  }
  return fixture;
}

/**
 * Creates one disposable userData root per Electron suite. An optional,
 * separately managed tool fixture is copied into the profile; runtime startup
 * still verifies its signed manifests and tree hashes before using it.
 */
export function createE2EProfile(label, options = {}, {
  copyDirectory = cpSync,
  createTemporaryDirectory = mkdtempSync,
  removeDirectory = rmSync,
} = {}) {
  // Resolve and validate external input before allocating anything owned by
  // this call. A bad fixture must not leave an otherwise unreachable profile.
  const fixture = verifiedFixtureDirectory(
    options.toolFixture ?? process.env[E2E_TOOL_FIXTURE_ENV],
  );
  const root = createTemporaryDirectory(join(tmpdir(), `cliphub-${safeLabel(label)}-`));
  try {
    if (fixture !== null) {
      copyDirectory(fixture, join(root, 'tools'), {
        dereference: false,
        errorOnExist: true,
        recursive: true,
        verbatimSymlinks: true,
      });
    }
  } catch (error) {
    try {
      removeDirectory(root, { force: true, recursive: true });
    } catch (cleanupError) {
      throw new AggregateError(
        [error, cleanupError],
        'E2E profile initialization failed and its temporary root could not be removed',
      );
    }
    throw error;
  }

  let disposed = false;
  return {
    root,
    environment(base = process.env) {
      return { ...base, [E2E_USER_DATA_ENV]: root };
    },
    dispose() {
      if (disposed) return;
      removeDirectory(root, { force: true, recursive: true });
      disposed = true;
    },
  };
}

export function profileHasCopiedToolFixture(profileRoot) {
  const tools = join(resolve(profileRoot), 'tools');
  return existsSync(tools) && basename(tools) === 'tools';
}
