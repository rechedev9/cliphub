import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const repo = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const desktopVersion = JSON.parse(readFileSync(join(repo, 'desktop', 'package.json'), 'utf8')).version;
const site = readFileSync(join(repo, 'landing', 'lib', 'site.ts'), 'utf8');

function siteConstant(name) {
  const match = site.match(new RegExp(`export const ${name} =\\s*"([^"]+)"`));
  assert.ok(match, `landing/lib/site.ts must define ${name}`);
  return match[1];
}

test('the landing offers the Studio version this release builds', () => {
  assert.equal(siteConstant('RELEASE_VERSION'), `v${desktopVersion}`);
  assert.equal(
    siteConstant('DOWNLOAD_URL'),
    `https://github.com/rechedev9/cliphub/releases/download/v${desktopVersion}/ClipHub.Studio.Setup.${desktopVersion}.exe`,
  );
});
