import assert from 'node:assert/strict';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
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

function workspacePackages() {
  const manifest = readFileSync(join(repoRoot, 'pnpm-workspace.yaml'), 'utf8');
  const block = manifest.split(/\r?\n(?=\S)/).find((section) => section.startsWith('packages:'));
  assert.ok(block, 'pnpm-workspace.yaml lists no packages');
  return [...block.matchAll(/^\s+-\s+["']?([^"'\s]+)["']?\s*$/gm)].map((match) => match[1]);
}

test('every installed minimatch can expand brace globs with its compatible dependency API', () => {
  const rootModules = join(repoRoot, 'node_modules');
  assert.equal(existsSync(rootModules), true, `workspace install missing: ${rootModules}`);
  const installations = [rootModules, ...workspacePackages().map((pkg) => join(repoRoot, pkg, 'node_modules'))]
    .flatMap(installedMinimatches);
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
