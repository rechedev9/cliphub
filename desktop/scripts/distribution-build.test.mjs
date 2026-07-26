import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { runDistributionBuildSteps } from './distribution-build.mjs';

test('rebuilds Go runtimes before desktop build and resource assembly', () => {
  const repo = join('C:', 'work', 'fragforge');
  const desktop = join(repo, 'desktop');
  const environment = { KEEP_ME: 'yes' };
  const calls = [];

  runDistributionBuildSteps({
    repo,
    desktop,
    environment,
    runFile(command, args, options) {
      calls.push({ kind: 'file', command, args, options });
    },
    runShell(command, options) {
      calls.push({ kind: 'shell', command, options });
    },
  });

  assert.deepEqual(calls.map(({ kind, command }) => `${kind}:${command}`), [
    'file:powershell.exe',
    'shell:pnpm run build',
    'shell:node scripts/assemble.mjs',
  ]);
  assert.deepEqual(calls[0].args, [
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-File',
    join(repo, 'scripts', 'build.ps1'),
  ]);
  for (const call of calls) {
    assert.equal(call.options.env, environment);
    assert.equal(call.options.stdio, 'inherit');
  }
  assert.equal(calls[0].options.cwd, repo);
  assert.equal(calls[1].options.cwd, desktop);
  assert.equal(calls[2].options.cwd, desktop);
});

test('distribution entrypoint cannot package before the guarded runtime rebuild', () => {
  const source = readFileSync(new URL('./dist.mjs', import.meta.url), 'utf8');
  const rebuild = source.indexOf(
    'runDistributionBuildSteps({ repo, desktop, environment: sanitizedEnvironment });',
  );
  const packageInstaller = source.indexOf("execSync('electron-builder --win nsis'");

  assert.notEqual(rebuild, -1, 'dist.mjs must invoke the guarded runtime rebuild');
  assert.notEqual(packageInstaller, -1, 'dist.mjs must retain its installer build');
  assert.ok(rebuild < packageInstaller, 'Go runtimes must be rebuilt before electron-builder packages them');
});
