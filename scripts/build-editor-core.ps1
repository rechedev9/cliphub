$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not $root) { $root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path }
$crate = Join-Path $root "editor-core"
Push-Location $crate
try {
    cargo test --offline 2>$null
    if ($LASTEXITCODE -ne 0) {
        cargo test
    }
} finally {
    Pop-Location
}
