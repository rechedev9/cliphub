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
const workflow = readFileSync(workflowPath, 'utf8').replaceAll('\r\n', '\n');
const jobs = Object.fromEntries(
  workflow
    .slice(workflow.indexOf('\njobs:\n'))
    .split(/\n(?= {2}[\w-]+:\n)/)
    .slice(1)
    .map((block) => [block.trim().split(':')[0], block]),
);

test('desktop release workflow is the unsigned windows-latest publisher', () => {
  const required = [
    ['name: Desktop release', 'keep the single hosted pipeline name stable'],
    ['runs-on: windows-latest', 'NSIS packaging only works on Windows'],
    ['workflow_dispatch:', 'next unsigned cut must be triggerable without a local PC'],
    ["- 'v*.*.*'", 'pushing a matching version tag must cut the same release'],
    ['CSC_IDENTITY_AUTO_DISCOVERY: \'false\'', 'CI must stay unsigned'],
    ['go-version-file: go.mod', 'CI must install the go.mod toolchain for the NSIS rebuild'],
    ['GOTOOLCHAIN: local', 'windows-latest must rebuild zv with the installed 1.26.10, not auto-download'],
    ['node desktop/scripts/seed-bundled-music.mjs', 'CI must seed gitignored Suno audio from the last installer'],
    ['pnpm --dir desktop run dist', 'must reuse the existing dist entrypoint'],
    ['FACEIT_API_KEY: ${{ secrets.FACEIT_API_KEY }}', 'CI must embed FACEIT in zv-orchestrator.exe'],
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
    ['--clobber', 'a published asset must never be overwritten'],
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

test('desktop release workflow checks, builds on windows and publishes in a separate job', () => {
  assert.deepEqual(Object.keys(jobs), ['checks', 'windows-installer', 'publish']);
});

test('a release publishes only after the checks pass', () => {
  const checks = jobs.checks;
  assert.match(checks, /contents: read/);
  assert.doesNotMatch(checks, /contents: write/);
  for (const command of ['go vet ./...', 'go test -short ./...', 'go run ./cmd/zv check']) {
    assert.ok(checks.includes(command), `checks must run ${command}`);
  }
  for (const pkg of ['web', 'desktop']) {
    for (const script of ['typecheck', 'lint', 'test:unit']) {
      assert.ok(checks.includes(`pnpm --dir ${pkg} run ${script}`), `checks must run ${pkg} ${script}`);
    }
  }
  assert.match(jobs.publish, /needs: \[checks, windows-installer\]/);
});

test('only the publish job can write releases', () => {
  assert.match(workflow, /^permissions: \{\}$/m, 'the workflow default token must have no scope');

  const build = jobs['windows-installer'];
  assert.match(build, /contents: read/, 'the build job runs third-party code with a read-only token');
  assert.doesNotMatch(build, /contents: write/);
  assert.doesNotMatch(build, /gh release/, 'the build job must not publish');

  const publish = jobs.publish;
  assert.match(publish, /environment: release/, 'publish must run inside the protected release environment');
  assert.match(publish, /contents: write/);
  assert.deepEqual(
    [...publish.matchAll(/uses:\s*([^@\s]+)@/g)].map((match) => match[1]),
    ['actions/download-artifact'],
    'publish must not check out or run project code',
  );
  assert.doesNotMatch(publish, /^\s*(?:run: )?(?:pnpm|npm|npx|node|bun|go) /m, 'publish must not run project code');
});

test('every action in the release workflow is pinned to a full commit SHA', () => {
  for (const [, ref] of workflow.matchAll(/uses:\s*(\S+)/g)) {
    assert.match(ref, /@[0-9a-f]{40}$/, `${ref} must be pinned by SHA`);
  }
});
