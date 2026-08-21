$ErrorActionPreference = "Stop"

$repo = Split-Path -Parent $PSScriptRoot
$cursorAssets = Join-Path $env:USERPROFILE ".cursor\projects\c-Users-reche-Documents-Projects-tickcut\assets"
$pkg = Join-Path $repo "internal\keydropbanner"
$public = Join-Path $repo "web\public\brand\keydrop"

New-Item -ItemType Directory -Force -Path $public | Out-Null

$generated = @{
    "style-tigerr.png" = "style-tigerr.png"
    "style-jcorko.png" = "style-jcorko.png"
}
foreach ($name in $generated.Keys) {
    $src = Join-Path $cursorAssets $name
    if (-not (Test-Path -LiteralPath $src)) {
        throw "missing generated plate $src"
    }
    Copy-Item -LiteralPath $src -Destination (Join-Path $pkg $name) -Force
}

$publicNames = @{
    "style-operator.png" = "operator.png"
    "style-classic.png"  = "classic.png"
    "style-tigerr.png"   = "tigerr.png"
    "style-jcorko.png"   = "jcorko.png"
}
foreach ($srcName in $publicNames.Keys) {
    $src = Join-Path $pkg $srcName
    if (-not (Test-Path -LiteralPath $src)) {
        throw "missing package plate $src"
    }
    Copy-Item -LiteralPath $src -Destination (Join-Path $public $publicNames[$srcName]) -Force
}

Write-Host "synced KeyDrop plates to $pkg and $public"
