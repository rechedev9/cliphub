import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const repo = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const workflowsDir = join(repo, '.github', 'workflows');

const ciWorkflows = ['ci-backend.yml', 'ci-frontend.yml', 'ci-infra.yml'];

function readWorkflow(name) {
  return readFileSync(join(workflowsDir, name), 'utf8');
}

test('hosted workflows are the three CI lanes plus the unsigned release job', () => {
  const names = readdirSync(workflowsDir)
    .filter((name) => name.endsWith('.yml') || name.endsWith('.yaml'))
    .sort();
  assert.deepEqual(names, [...ciWorkflows, 'desktop-release.yml']);
});

test('CI lanes are contents:read and never cut or sign the installer', () => {
  for (const name of ciWorkflows) {
    const body = readWorkflow(name);
    assert.match(body, /^permissions:\n  contents: read$/m, `${name} must be contents: read only`);
    assert.equal(body.includes('contents: write'), false, `${name} must not request contents: write`);
    assert.equal(body.includes('pnpm --dir desktop run dist'), false, `${name} must not run dist`);
    assert.equal(body.includes('test:e2e'), false, `${name} must not run Playwright or Electron GUI e2e`);
    assert.equal(body.includes('windows-latest'), false, `${name} is not the Windows installer job`);
    assert.equal(body.toLowerCase().includes('signtool'), false, `${name} must stay unsigned`);
    assert.equal(body.toLowerCase().includes('authenticode'), false, `${name} must stay unsigned`);
    assert.equal(body.toLowerCase().includes('vercel'), false, `${name} is not a landing deploy`);
  }
});

test('only desktop-release.yml may build the unsigned installer', () => {
  const distFiles = readdirSync(workflowsDir)
    .filter((name) => name.endsWith('.yml') || name.endsWith('.yaml'))
    .filter((name) => readWorkflow(name).includes('pnpm --dir desktop run dist'));
  assert.deepEqual(distFiles, ['desktop-release.yml']);
});

test('desktop package version is semver x.y.z', () => {
  const version = JSON.parse(readFileSync(join(repo, 'desktop', 'package.json'), 'utf8')).version;
  assert.match(version, /^\d+\.\d+\.\d+$/);
});

test('CI frontend pins Node 24, pnpm 11.22.0, and the unit/typecheck/lint commands', () => {
  const body = readWorkflow('ci-frontend.yml');
  assert.match(body, /node-version:\s*24/);
  assert.match(body, /version:\s*11\.22\.0/);
  assert.match(body, /pnpm --dir web install --frozen-lockfile/);
  assert.match(body, /pnpm --dir desktop install --frozen-lockfile/);
  assert.match(body, /pnpm --dir web run typecheck/);
  assert.match(body, /pnpm --dir web run lint/);
  assert.match(body, /pnpm --dir web run test:unit/);
  assert.match(body, /pnpm --dir desktop run typecheck/);
  assert.match(body, /pnpm --dir desktop run lint/);
  assert.match(body, /pnpm --dir desktop run test:unit/);
  assert.match(body, /ELECTRON_SKIP_BINARY_DOWNLOAD/);
  assert.equal(body.includes('pnpm --dir landing'), false, 'landing is not part of Studio CI');
});

test('CI backend pins go.mod and runs vet, test, and zv check', () => {
  const body = readWorkflow('ci-backend.yml');
  assert.match(body, /go-version-file:\s*go\.mod/);
  assert.match(body, /GOTOOLCHAIN:\s*local/);
  assert.match(body, /go vet \.\/\.\.\./);
  assert.match(body, /go test \.\/\.\.\. -count=1 -timeout 3m/);
  assert.match(body, /go run \.\/cmd\/zv check/);
});
