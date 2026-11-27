import assert from 'node:assert/strict';
import { existsSync, readdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');

function installedMinimatches(modulesDirectory) {
  if (!existsSync(modulesDirectory)) return [];
  const found = [];
  for (const entry of readdirSync(modulesDirectory, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const entryPath = join(modulesDirectory, entry.name);
    if (entry.name.startsWith('@')) {
      found.push(...installedMinimatches(entryPath));
      continue;
    }
    if (entry.name === 'minimatch') found.push(join(entryPath, 'package.json'));
    found.push(...installedMinimatches(join(entryPath, 'node_modules')));
  }
  return found.filter(existsSync);
}

test('every installed minimatch can expand brace globs with its compatible dependency API', () => {
  const rootModules = join(repoRoot, 'node_modules');
  assert.equal(existsSync(rootModules), true, `workspace install missing: ${rootModules}`);
  const installations = installedMinimatches(rootModules);
  assert.ok(installations.length > 0, 'no minimatch installations found');

  for (const packageJSON of installations) {
    const require = createRequire(packageJSON);
    const loaded = require('minimatch');
    const minimatch = typeof loaded === 'function' ? loaded : loaded.minimatch;
    assert.equal(typeof minimatch, 'function', `${packageJSON} has no callable minimatch export`);
    assert.equal(minimatch('clip.mp4', '{clip,cover}.mp4'), true, packageJSON);
    assert.equal(minimatch('caption.txt', '{clip,cover}.mp4'), false, packageJSON);
  }
});
