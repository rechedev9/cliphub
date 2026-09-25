# Pins this shell to the packaged HLAE unpack named by desktop/src/hlae-tool.json
# and prints capture readiness. Identity is the unpacked path + Test-Path. Do not
# Get-FileHash HLAE.exe against the manifest: sha256 is the zip, treeSha256 is
# Studio's tree digest.
$ErrorActionPreference = 'Stop'
$manifest = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\desktop\src\hlae-tool.json') | ConvertFrom-Json
$pin = Join-Path $env:APPDATA "cliphub-studio\tools\hlae\$($manifest.version)\$($manifest.exeRel)"
if (-not (Test-Path -LiteralPath $pin)) {
    throw "HLAE pin missing: $pin"
}
$env:ZV_HLAE_PATH = $pin
if (-not $env:ZV_CS2_PATH) {
    $env:ZV_CS2_PATH = 'C:\Program Files (x86)\Steam\steamapps\common\Counter-Strike Global Offensive\game\bin\win64\cs2.exe'
}
Write-Host "ZV_HLAE_PATH=$env:ZV_HLAE_PATH"
Write-Host "ZV_CS2_PATH=$env:ZV_CS2_PATH"
$cs2 = Get-Process cs2 -ErrorAction SilentlyContinue
if ($cs2) {
    Write-Host "cs2.exe is already running (pid $($cs2.Id)); close it before record"
} else {
    Write-Host "cs2.exe is not running"
}
$zv = Join-Path $PSScriptRoot '..\bin\zv.exe'
if (Test-Path -LiteralPath $zv) {
    & $zv capabilities --format json
}
