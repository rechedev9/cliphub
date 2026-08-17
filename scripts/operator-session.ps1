# Pins this shell to the packaged HLAE 2.192.1 unpack and prints capture readiness.
# Identity is the unpacked path + Test-Path. Do not Get-FileHash HLAE.exe against
# desktop/src/hlae-tool.json: sha256 is the zip, treeSha256 is Studio's tree digest.
$ErrorActionPreference = 'Stop'
$pin = Join-Path $env:APPDATA 'tickcut-studio\tools\hlae\2.192.1\HLAE.exe'
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
