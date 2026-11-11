import assert from 'node:assert/strict';
import { existsSync, readdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const desktopRoot = join(dirname(fileURLToPath(import.meta.url)), '..');

/** Every copy of `name` in a hoisted tree: the top one and each nested under a dependent. */
function installedCopies(modules, name) {
  if (!existsSync(modules)) return [];
  const found = [];
  for (const entry of readdirSync(modules, { withFileTypes: true })) {
    if (!entry.isDirectory() || entry.name.startsWith('.')) continue;
    const packages = entry.name.startsWith('@')
      ? readdirSync(join(modules, entry.name), { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => join(modules, entry.name, e.name))
      : [join(modules, entry.name)];
    for (const dir of packages) {
      if (dir.endsWith(`${join('node_modules', name)}`) && existsSync(join(dir, 'package.json'))) found.push(join(dir, 'package.json'));
      found.push(...installedCopies(join(dir, 'node_modules'), name));
    }
  }
  return found;
}

test('every installed minimatch can expand brace globs with its compatible dependency API', () => {
  const installations = installedCopies(join(desktopRoot, 'node_modules'), 'minimatch');
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
