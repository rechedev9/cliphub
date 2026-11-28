import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { runDistributionBuildSteps } from './distribution-build.mjs';

test('rebuilds Go runtimes before desktop build and resource assembly', () => {
  const repo = join('C:', 'work', 'cliphub');
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
    '-RequireFaceitEmbed',
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
    'runDistributionBuildSteps({ repo, desktop, environment: goEnvironment });',
  );
  const packageInstaller = source.indexOf('electronBuilderCli,');

  assert.notEqual(rebuild, -1, 'dist.mjs must invoke the guarded runtime rebuild');
  assert.notEqual(packageInstaller, -1, 'dist.mjs must retain its installer build');
  assert.ok(rebuild < packageInstaller, 'Go runtimes must be rebuilt before electron-builder packages them');
});

test('distribution entrypoint locks and recovers before starting a rebuild', () => {
  const source = readFileSync(new URL('./dist.mjs', import.meta.url), 'utf8');
  const acquire = source.indexOf(
    'releasePublicationLock = await acquirePublicationLock(canonicalRelease.directory);',
  );
  const recover = source.indexOf(
    'recoverInterruptedPublication(canonicalRelease.directory);',
  );
  const rebuild = source.indexOf(
    'runDistributionBuildSteps({ repo, desktop, environment: goEnvironment });',
  );
  const commit = source.indexOf(
    'commitPublishedDirectory(canonicalRelease.directory);',
  );
  const release = source.indexOf(
    'if (releasePublicationLock !== undefined) await releasePublicationLock();',
  );

  assert.notEqual(acquire, -1, 'dist.mjs must acquire the publication lock');
  assert.notEqual(recover, -1, 'dist.mjs must recover interrupted publication before rebuilding');
  assert.notEqual(rebuild, -1, 'dist.mjs must invoke the guarded runtime rebuild');
  assert.notEqual(commit, -1, 'dist.mjs must commit the verified publication');
  assert.notEqual(release, -1, 'dist.mjs must release the publication lock');
  assert.ok(acquire < recover, 'recovery must happen under the exclusive lock');
  assert.ok(recover < rebuild, 'recovery must happen before the long distribution build');
  assert.ok(commit < release, 'the publication lock must remain held through commit');
});

test('assemble stages cliphub-render.exe alongside Studio Go binaries', () => {
  const source = readFileSync(new URL('./assemble.mjs', import.meta.url), 'utf8');
  assert.match(source, /const cliphubRender = join\(bin, 'cliphub-render\.exe'\);/);
  assert.match(source, /\[zvOrchestrator, zvEditor, zvRecorder, zvMcp, cliphubRender\]/);
  assert.match(source, /cpSync\(cliphubRender, join\(out, 'bin', 'cliphub-render\.exe'\)\);/);
});

test('build.ps1 builds and atomically stages cliphub-render.exe without Go VCS stamping it', () => {
  const source = readFileSync(new URL('../../scripts/build.ps1', import.meta.url), 'utf8');
  const cargoBuild = source.search(/^\s+& cargo build --release --manifest-path \$renderManifest\s*$/m);
  const goStamp = source.search(/^\s+\$stamp = Assert-VcsRevisionStamped -Path \$out\s*$/m);
  const publish = source.search(/^\s+Publish-BuildArtifacts\b/m);
  assert.notEqual(cargoBuild, -1, 'build.ps1 must build tools/render with cargo --release');
  assert.notEqual(goStamp, -1, 'build.ps1 must stamp Go binaries');
  assert.notEqual(publish, -1, 'build.ps1 must publish artifacts atomically');
  assert.ok(goStamp < cargoBuild && cargoBuild < publish, 'cliphub-render must build after Go VCS stamping and before atomic publication');
  assert.match(source, /\$RequireFaceitEmbed -and -not \$hasCargo/);
  assert.match(source, /\$publishNames\.Add\("cliphub-render"\)/);
});
