import assert from 'node:assert/strict';
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import {
  createE2EProfile,
  E2E_USER_DATA_ENV,
  profileHasCopiedToolFixture,
} from './e2e-profile.mjs';

test('allocates independent disposable profiles for concurrent suites', (t) => {
  const first = createE2EProfile('ui');
  const second = createE2EProfile('ui');
  t.after(() => first.dispose());
  t.after(() => second.dispose());
  assert.notEqual(first.root, second.root);
  assert.equal(first.environment({ SAFE: 'yes' })[E2E_USER_DATA_ENV], first.root);
  assert.equal(first.environment({ SAFE: 'yes' }).SAFE, 'yes');
});

test('copies a managed tool fixture rather than sharing its mutable directory', (t) => {
  const fixture = mkdtempSync(join(tmpdir(), 'fragforge-tool-fixture-'));
  t.after(() => rmSync(fixture, { force: true, recursive: true }));
  mkdirSync(join(fixture, 'ffmpeg'), { recursive: true });
  writeFileSync(join(fixture, 'ffmpeg', 'fixture.txt'), 'verified by runtime on boot');

  const profile = createE2EProfile('fixture', { toolFixture: fixture });
  t.after(() => profile.dispose());
  assert.equal(profileHasCopiedToolFixture(profile.root), true);
  assert.notEqual(join(profile.root, 'tools'), fixture);
});

test('rejects an invalid fixture before allocating a profile root', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'fragforge-invalid-fixture-'));
  const fixtureFile = join(testRoot, 'not-a-directory');
  t.after(() => rmSync(testRoot, { force: true, recursive: true }));
  writeFileSync(fixtureFile, 'invalid fixture');
  let allocationCalled = false;

  assert.throws(
    () => createE2EProfile('invalid-fixture', { toolFixture: fixtureFile }, {
      createTemporaryDirectory() {
        allocationCalled = true;
        throw new Error('profile allocation must not run');
      },
    }),
    /FRAGFORGE_E2E_TOOL_FIXTURE must name a real directory/,
  );
  assert.equal(allocationCalled, false);
});

test('removes the allocated profile root when fixture copy initialization fails', (t) => {
  const fixture = mkdtempSync(join(tmpdir(), 'fragforge-copy-failure-fixture-'));
  t.after(() => rmSync(fixture, { force: true, recursive: true }));
  writeFileSync(join(fixture, 'fixture.txt'), 'fixture');
  let allocatedRoot;

  assert.throws(
    () => createE2EProfile('copy-failure', { toolFixture: fixture }, {
      createTemporaryDirectory(prefix) {
        allocatedRoot = mkdtempSync(prefix);
        return allocatedRoot;
      },
      copyDirectory() {
        throw new Error('injected fixture copy failure');
      },
    }),
    /injected fixture copy failure/,
  );
  assert.notEqual(allocatedRoot, undefined);
  assert.equal(existsSync(allocatedRoot), false);
});

test('retries profile disposal after a removal failure', (t) => {
  let removalAttempts = 0;
  const profile = createE2EProfile('retry-dispose', {}, {
    removeDirectory(path, options) {
      removalAttempts += 1;
      if (removalAttempts === 1) throw new Error('injected profile removal failure');
      rmSync(path, options);
    },
  });
  t.after(() => rmSync(profile.root, { force: true, recursive: true }));

  assert.throws(() => profile.dispose(), /injected profile removal failure/);
  assert.equal(existsSync(profile.root), true);
  profile.dispose();
  profile.dispose();

  assert.equal(removalAttempts, 2);
  assert.equal(existsSync(profile.root), false);
});
