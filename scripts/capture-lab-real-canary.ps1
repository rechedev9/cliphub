[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$KillPlan,
    [Parameter(Mandatory)] [string]$Demo,
    [Parameter(Mandatory)] [string]$OutDir,
    [Parameter(Mandatory)] [string]$HLAE,
    [Parameter(Mandatory)] [string]$CS2,
    [Parameter(Mandatory)] [string]$HLAEVersion,
    [Parameter(Mandatory)] [string]$LatestHLAEVersion,
    [Parameter(Mandatory)] [string]$CS2Build,
    [Parameter(Mandatory)] [ValidateSet('gameplay', 'clean', 'deathnotices')] [string]$HUD,
    [Parameter(Mandatory)] [bool]$PortraitSafeKillfeed,
    [ValidateSet('libx264', 'nvenc-h264', 'amf-h264', 'qsv-h264')] [string]$Encoder = 'libx264',
    [ValidateRange(1, 240)] [int]$FPS = 60,
    [ValidateRange(1, 51)] [int]$VideoCRF = 18,
    [ValidateRange(1.0, 32.0)] [double]$GapTimescale = 8,
    [ValidateRange(0.1, 10.0)] [double]$SettleSeconds = 2,
    [ValidateRange(60, 7200)] [int]$TimeoutSeconds = 1800,
    [Parameter(Mandatory)] [switch]$ApproveRealCapture
)

$ErrorActionPreference = 'Stop'
if (-not $ApproveRealCapture) {
    throw 'Real capture was not explicitly approved. Review the exact HUD/profile brief and pass -ApproveRealCapture.'
}
if ($HLAEVersion -ne $LatestHLAEVersion) {
    throw "Installed HLAE $HLAEVersion is not the latest checked official release $LatestHLAEVersion. Update or review explicitly before capture."
}
if ($HLAE -ieq 'C:\HLAE\HLAE.exe') {
    throw 'C:\HLAE\HLAE.exe is forbidden for ClipHub capture. Use the highest installed C:\HLAE-*\HLAE.exe.'
}
foreach ($item in @($KillPlan, $Demo, $HLAE, $CS2)) {
    if (-not (Test-Path -LiteralPath $item -PathType Leaf)) {
        throw "Required input not found: $item"
    }
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
$Build = Join-Path $PSScriptRoot 'build.ps1'
$ZV = Join-Path $RepoRoot 'bin/zv.exe'
$Portrait = $PortraitSafeKillfeed.ToString().ToLowerInvariant()
$recordArgs = @(
    'record',
    '--killplan', $KillPlan,
    '--demo', $Demo,
    '--out', $OutDir,
    '--hlae', $HLAE,
    '--cs2', $CS2,
    '--hud', $HUD,
    "--portrait-safe-killfeed=$Portrait",
    '--fps', [string]$FPS,
    '--video-crf', [string]$VideoCRF,
    '--encoder', $Encoder,
    '--gap-timescale', [string]$GapTimescale,
    '--settle-seconds', [string]$SettleSeconds,
    '--timeout', "${TimeoutSeconds}s",
    '--format', 'json'
)

Push-Location $RepoRoot
try {
    # The canary must exercise binaries rebuilt from the current source. A
    # pre-existing bin/zv.exe is never accepted as evidence of source identity.
    & $Build
    if ($LASTEXITCODE -ne 0) { throw "build failed with exit code $LASTEXITCODE" }

    $displayCommand = @($ZV) + $recordArgs | ForEach-Object {
        if ($_ -match '[\s"]') { '"' + ($_ -replace '"', '\"') + '"' } else { $_ }
    }
    $exactCommand = $displayCommand -join ' '
    $argvPath = Join-Path $OutDir 'capture-canary-argv.json'
    $exactArgv = @($ZV) + $recordArgs
    [System.IO.Directory]::CreateDirectory($OutDir) | Out-Null
    [System.IO.File]::WriteAllText(
        $argvPath,
        ($exactArgv | ConvertTo-Json -Depth 3),
        [System.Text.UTF8Encoding]::new($false)
    )
    Write-Host "REAL CAPTURE CANARY: $exactCommand"
    & $ZV @recordArgs
    if ($LASTEXITCODE -ne 0) { throw "real capture failed with exit code $LASTEXITCODE" }

    $result = Join-Path $OutDir 'recording-result.json'
    $certificate = Join-Path $OutDir 'capture-compatibility-certificate.json'
    & node (Join-Path $PSScriptRoot 'capturelab/certificate.mjs') issue `
        --recording-result $result `
        --hlae-version $HLAEVersion `
        --cs2-build $CS2Build `
        --argv-json $argvPath `
        --out $certificate
    if ($LASTEXITCODE -ne 0) { throw "certificate issue failed with exit code $LASTEXITCODE" }
    Write-Host "Compatibility certificate: $certificate"
}
finally {
    Pop-Location
}
