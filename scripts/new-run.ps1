# Creates a dated ClipHub output run and prints the paths to pass to zv.
# Root: %USERPROFILE%\Videos\ClipHub\{ready,runs}
# Usage:
#   .\scripts\new-run.ps1 mahar-anubis
#   .\scripts\new-run.ps1 -Name fut-mirage-xertion
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Name
)

$ErrorActionPreference = 'Stop'
$root = Join-Path $env:USERPROFILE 'Videos\ClipHub'
$ready = Join-Path $root 'ready'
$runs = Join-Path $root 'runs'
$slug = ($Name.ToLowerInvariant() -replace '[^a-z0-9]+', '-').Trim('-')
if (-not $slug) {
    throw 'run name must contain letters or digits'
}
$day = Get-Date -Format 'yyyy-MM-dd'
$dir = Join-Path $runs "$day-$slug"
New-Item -ItemType Directory -Force -Path $ready, (Join-Path $dir 'shortslistosparasubir') | Out-Null

Write-Host "run=$dir"
Write-Host "publish=$(Join-Path $dir 'shortslistosparasubir')"
Write-Host "ready=$ready"
Write-Host "record --out $(Join-Path $dir 'recording')"
Write-Host "shorts render --out $(Join-Path $dir 'render') --publish-dir $(Join-Path $dir 'shortslistosparasubir')"
