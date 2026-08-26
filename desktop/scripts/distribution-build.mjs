import { execFileSync, execSync } from 'node:child_process';
import { join } from 'node:path';

/**
 * Rebuilds every source-derived runtime before staging desktop resources.
 * `bin/` is an artifact cache, never evidence that its executables match the
 * current Go source, so the release path must refresh it unconditionally.
 */
export function runDistributionBuildSteps({
  repo,
  desktop,
  environment,
  runFile = execFileSync,
  runShell = execSync,
}) {
  runFile(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      join(repo, 'scripts', 'build.ps1'),
      '-RequireFaceitEmbed',
    ],
    { cwd: repo, env: environment, stdio: 'inherit' },
  );
  runShell('pnpm run build', { cwd: desktop, env: environment, stdio: 'inherit' });
  runShell('node scripts/assemble.mjs', {
    cwd: desktop,
    env: environment,
    stdio: 'inherit',
  });
}
