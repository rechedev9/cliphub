[CmdletBinding()]
param(
    [ValidateSet('Script', 'Media', 'App', 'Studio', 'Full')]
    [string]$Mode = 'Full',

    [ValidateRange(1, 100)]
    [int]$Iterations = 1,

    [ValidateRange(1, 3600)]
    [int]$TimeoutSeconds = 180,

    [string]$EvidenceDir = ''
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$Runner = Join-Path $PSScriptRoot 'capturelab/lab.mjs'
if (-not (Test-Path -LiteralPath $Runner -PathType Leaf)) {
    throw "Capture Lab runner not found: $Runner"
}

$arguments = @(
    $Runner,
    '--mode', $Mode,
    '--iterations', [string]$Iterations,
    '--timeout-seconds', [string]$TimeoutSeconds
)
if ($EvidenceDir) {
    $arguments += @('--evidence-dir', $EvidenceDir)
}

Push-Location $RepoRoot
try {
    & node @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Capture Lab failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
