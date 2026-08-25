import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const workflowPath = join(
  dirname(fileURLToPath(import.meta.url)),
  '..',
  '..',
  '.github',
  'workflows',
  'desktop-release.yml',
);
const workflow = readFileSync(workflowPath, 'utf8');

test('desktop release workflow is the unsigned windows-latest publisher', () => {
  const required = [
    ['name: Desktop release', 'keep the single hosted pipeline name stable'],
    ['runs-on: windows-latest', 'NSIS packaging only works on Windows'],
    ['workflow_dispatch:', 'next unsigned cut must be triggerable without a local PC'],
    ["- 'v*.*.*'", 'pushing a matching version tag must cut the same release'],
    ['permissions:', 'GITHUB_TOKEN scope must be explicit'],
    ['contents: write', 'release asset upload needs contents: write'],
    ['CSC_IDENTITY_AUTO_DISCOVERY: \'false\'', 'CI must stay unsigned'],
    ['pnpm --dir desktop run dist', 'must reuse the existing dist entrypoint'],
    ['pnpm --dir desktop run verify:dist-integrity', 'must verify SHA256SUMS.txt before publish'],
    ['ClipHub.Studio.Setup.$version.exe', 'updater reads this installer name'],
    ['SHA256SUMS.txt', 'updater verifies the installer digest'],
    ['gh release create', 'must publish a stable GitHub Release'],
    ['--latest', 'Actualizar reads releases/latest'],
  ];

  for (const [needle, why] of required) {
    assert.notEqual(workflow.indexOf(needle), -1, `${why}: missing ${needle}`);
  }

  const forbidden = [
    ['signtool', 'must not add Authenticode signing'],
    ['authenticode', 'must not add Authenticode signing'],
    ['electron-builder --publish', 'must not invent a second publish path'],
    ['vercel', 'Actualizar does not depend on a landing deploy'],
  ];
  for (const [needle, why] of forbidden) {
    assert.equal(workflow.toLowerCase().includes(needle), false, why);
  }

  assert.equal(
    (workflow.match(/^jobs:/gm) ?? []).length,
    1,
    'do not invent a second pipeline job map',
  );
});

test('desktop release workflow has one windows installer job', () => {
  const jobsBlock = workflow.slice(workflow.indexOf('jobs:'));
  assert.match(jobsBlock, /^\s{2}windows-installer:\s*$/m);
  assert.deepEqual(
    [...jobsBlock.matchAll(/^\s{2}[A-Za-z0-9_-]+:\s*$/gm)].map((match) => match[0].trim()),
    ['windows-installer:'],
  );
});
