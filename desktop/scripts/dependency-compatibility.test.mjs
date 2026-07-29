import assert from 'node:assert/strict';
import { existsSync, readdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const desktopRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const virtualStore = join(desktopRoot, 'node_modules', '.pnpm');

test('every installed minimatch can expand brace globs with its compatible dependency API', () => {
  assert.equal(existsSync(virtualStore), true, `pnpm virtual store missing: ${virtualStore}`);
  const installations = readdirSync(virtualStore)
    .filter((name) => name.startsWith('minimatch@'))
    .map((name) => join(virtualStore, name, 'node_modules', 'minimatch', 'package.json'))
    .filter(existsSync);
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
