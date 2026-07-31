import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import {
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const isWin32 = process.platform === 'win32';
const winTest = isWin32 ? test : test.skip;

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '..', '..');
const helperPath = join(repo, 'scripts', 'local-process-job.ps1');
const launcherPath = join(repo, 'scripts', 'local-studio.ps1');

test('Local Studio assigns suspended services before resuming them', () => {
  const helper = readFileSync(helperPath, 'utf8');
  const launcher = readFileSync(launcherPath, 'utf8');
  const assignedAt = helper.indexOf('AssignProcessToJobObject(job, processInfo.hProcess)');
  const handleAcquiredAt = helper.indexOf('process.Handle');
  const resumedAt = helper.indexOf('ResumeThread(processInfo.hThread)');

  assert.match(helper, /CREATE_SUSPENDED/);
  assert.ok(assignedAt >= 0);
  assert.ok(handleAcquiredAt > assignedAt);
  assert.ok(resumedAt > handleAcquiredAt);
  assert.ok(resumedAt > assignedAt);
  assert.equal(
    launcher.match(/\[TickCut\.LocalProcessJob\]::StartInJob\(/g)?.length,
    2,
  );
  assert.doesNotMatch(launcher, /LocalProcessJob\]::AddProcess/);
});

winTest('a returned short-lived process remains waitable with its exit code', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'tickcut-local-job-exit-'));
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));
  const harnessPath = join(testRoot, 'harness.ps1');

  writeFileSync(harnessPath, `
param(
    [Parameter(Mandatory = $true)][string]$HelperPath,
    [Parameter(Mandatory = $true)][string]$RunRoot
)
$ErrorActionPreference = "Stop"
. $HelperPath
$command = (Get-Command cmd.exe -ErrorAction Stop).Source
$job = [TickCut.LocalProcessJob]::CreateKillOnClose()
$process = $null
try {
    $process = [TickCut.LocalProcessJob]::StartInJob(
        $job,
        $command,
        "/d /c exit 37",
        $RunRoot
    )
    $processId = $process.Id
    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    while ((Get-Process -Id $processId -ErrorAction SilentlyContinue) -and
           [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 10
    }
    if (Get-Process -Id $processId -ErrorAction SilentlyContinue) {
        throw "short-lived child did not exit"
    }

    $process.WaitForExit()
    if ($process.ExitCode -ne 37) {
        throw "short-lived child exit code was $($process.ExitCode), want 37"
    }
} finally {
    if ($null -ne $process) { $process.Dispose() }
    [TickCut.LocalProcessJob]::Close($job)
}
`, 'utf8');

  const result = spawnSync('powershell.exe', [
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-File',
    harnessPath,
    helperPath,
    testRoot,
  ], {
    encoding: 'utf8',
    timeout: 15_000,
    windowsHide: true,
  });

  assert.equal(result.status, 0, result.stderr || result.stdout);
});

winTest('closing the Local Studio job kills a service and its descendant', (t) => {
  const testRoot = mkdtempSync(join(tmpdir(), 'tickcut-local-job-'));
  t.after(() => rmSync(testRoot, { recursive: true, force: true }));
  const grandchildPath = join(testRoot, 'grandchild.ps1');
  const servicePath = join(testRoot, 'service.ps1');
  const harnessPath = join(testRoot, 'harness.ps1');

  writeFileSync(grandchildPath, `
param([Parameter(Mandatory = $true)][string]$PidPath)
$PID | Set-Content -LiteralPath $PidPath
while ($true) { Start-Sleep -Milliseconds 100 }
`, 'utf8');
  writeFileSync(servicePath, `
param([Parameter(Mandatory = $true)][string]$RunRoot)
$grandchildScript = Join-Path $RunRoot "grandchild.ps1"
$childPidPath = Join-Path $RunRoot "child.pid"
$grandchild = Start-Process -FilePath "powershell.exe" -ArgumentList @(
    "-NoProfile",
    "-NonInteractive",
    "-File",
    ('"' + $grandchildScript + '"'),
    ('"' + $childPidPath + '"')
) -PassThru -WindowStyle Hidden
$PID | Set-Content -LiteralPath (Join-Path $RunRoot "service.pid")
$grandchild.Id | Set-Content -LiteralPath $childPidPath
"ready" | Set-Content -LiteralPath (Join-Path $RunRoot "ready")
while ($true) { Start-Sleep -Milliseconds 100 }
`, 'utf8');
  writeFileSync(harnessPath, `
param(
    [Parameter(Mandatory = $true)][string]$HelperPath,
    [Parameter(Mandatory = $true)][string]$RunRoot
)
$ErrorActionPreference = "Stop"
. $HelperPath
$powershell = (Get-Command powershell.exe -ErrorAction Stop).Source
for ($iteration = 0; $iteration -lt 10; $iteration++) {
    $iterationRoot = Join-Path $RunRoot "run-$iteration"
    New-Item -ItemType Directory -Path $iterationRoot | Out-Null
    Copy-Item -LiteralPath (Join-Path $RunRoot "grandchild.ps1") -Destination $iterationRoot
    Copy-Item -LiteralPath (Join-Path $RunRoot "service.ps1") -Destination $iterationRoot
    $serviceScript = Join-Path $iterationRoot "service.ps1"
    $arguments = '-NoProfile -NonInteractive -File "' + $serviceScript + '" "' + $iterationRoot + '"'
    $job = [TickCut.LocalProcessJob]::CreateKillOnClose()
    try {
        [void][TickCut.LocalProcessJob]::StartInJob(
            $job,
            $powershell,
            $arguments,
            $iterationRoot
        )
        $deadline = [DateTime]::UtcNow.AddSeconds(5)
        $readyPath = Join-Path $iterationRoot "ready"
        while (-not (Test-Path $readyPath)) {
            if ([DateTime]::UtcNow -ge $deadline) { throw "service did not become ready" }
            Start-Sleep -Milliseconds 20
        }
        $servicePid = [int](Get-Content (Join-Path $iterationRoot "service.pid"))
        $childPid = [int](Get-Content (Join-Path $iterationRoot "child.pid"))
    } finally {
        [TickCut.LocalProcessJob]::Close($job)
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    while ((Get-Process -Id $servicePid, $childPid -ErrorAction SilentlyContinue) -and
           [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 20
    }
    if (Get-Process -Id $servicePid, $childPid -ErrorAction SilentlyContinue) {
        throw "job close left a service descendant running"
    }
}
`, 'utf8');

  const result = spawnSync('powershell.exe', [
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-File',
    harnessPath,
    helperPath,
    testRoot,
  ], {
    encoding: 'utf8',
    timeout: 30_000,
    windowsHide: true,
  });

  assert.equal(result.status, 0, result.stderr || result.stdout);
});
