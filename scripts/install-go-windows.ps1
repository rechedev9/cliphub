# Pins and installs the official Windows Go toolchain so GOTOOLCHAIN=local can
# rebuild zv / zv-recorder. Used by scripts/build.ps1 (Studio installer rebuild)
# and scripts/check-toolchain.ps1. Does not require elevation: the zip lands in
# %LOCALAPPDATA%\ClipHub\toolchain\go<version>.
$ErrorActionPreference = "Stop"

function Read-PinnedWindowsGo {
    $pinPath = Join-Path $PSScriptRoot "go-windows.json"
    if (-not (Test-Path -LiteralPath $pinPath -PathType Leaf)) {
        throw "missing Windows Go pin: $pinPath"
    }
    $pin = Get-Content -LiteralPath $pinPath -Raw | ConvertFrom-Json
    if ($pin.version -notmatch '^\d+\.\d+\.\d+$') {
        throw "go-windows.json version is invalid"
    }
    if ($pin.archiveFilename -notmatch '^go\d+\.\d+\.\d+\.windows-amd64\.zip$') {
        throw "go-windows.json archiveFilename is invalid"
    }
    if ($pin.archiveUrl -ne "https://go.dev/dl/$($pin.archiveFilename)") {
        throw "go-windows.json archiveUrl must be the official go.dev URL"
    }
    if ($pin.archiveSha256 -notmatch '^[a-f0-9]{64}$') {
        throw "go-windows.json archiveSha256 is invalid"
    }
    if ($pin.url -ne "https://go.dev/dl/$($pin.filename)") {
        throw "go-windows.json url must be the official go.dev MSI URL"
    }
    if ($pin.sha256 -notmatch '^[a-f0-9]{64}$') {
        throw "go-windows.json sha256 is invalid"
    }
    return $pin
}

function ConvertTo-GoVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Raw
    )
    if ($Raw -notmatch '^\d+\.\d+(?:\.\d+)?$') {
        throw "invalid Go version: $Raw"
    }
    if ($Raw -notmatch '^\d+\.\d+\.\d+$') {
        $Raw = "$Raw.0"
    }
    return [version]$Raw
}

function Get-GoVersionFromOutput {
    param([string]$Output)
    $match = [regex]::Match([string]$Output, 'go version go(\d+\.\d+(?:\.\d+)?)')
    if (-not $match.Success) {
        return $null
    }
    return ConvertTo-GoVersion $match.Groups[1].Value
}

function Get-Sha256Hex {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $sha = [System.Security.Cryptography.SHA256]::Create()
        try {
            return [System.BitConverter]::ToString($sha.ComputeHash($stream)).Replace('-', '').ToLowerInvariant()
        } finally { $sha.Dispose() }
    } finally { $stream.Dispose() }
}

function Use-GoInstall {
    param(
        [Parameter(Mandatory = $true)]
        [string]$GoRoot
    )
    $bin = Join-Path $GoRoot "bin"
    $goExe = Join-Path $bin "go.exe"
    if (-not (Test-Path -LiteralPath $goExe -PathType Leaf)) {
        throw "pinned Go install is missing $goExe"
    }
    $env:PATH = "$bin;$env:PATH"
    $env:GOROOT = $GoRoot
    $env:GOTOOLCHAIN = "local"
}

function Install-PinnedWindowsGo {
    param(
        [string]$ToolsRoot = $(Join-Path $env:LOCALAPPDATA "ClipHub\toolchain")
    )
    $pin = Read-PinnedWindowsGo
    $required = ConvertTo-GoVersion $pin.version

    $pathGo = Get-Command go -ErrorAction SilentlyContinue
    if ($null -ne $pathGo) {
        $have = Get-GoVersionFromOutput (& go version 2>&1 | Select-Object -First 1)
        if ($null -ne $have -and $have -ge $required) {
            $env:GOTOOLCHAIN = "local"
            return
        }
    }

    $installDir = Join-Path $ToolsRoot "go$($pin.version)"
    $goRoot = Join-Path $installDir "go"
    $goExe = Join-Path $goRoot "bin\go.exe"
    $marker = Join-Path $installDir ".cliphub-go.json"
    if ((Test-Path -LiteralPath $goExe -PathType Leaf) -and (Test-Path -LiteralPath $marker -PathType Leaf)) {
        $cached = Get-Content -LiteralPath $marker -Raw | ConvertFrom-Json
        if ($cached.version -eq $pin.version -and $cached.archiveSha256 -eq $pin.archiveSha256) {
            $have = Get-GoVersionFromOutput (& $goExe version 2>&1 | Select-Object -First 1)
            if ($null -ne $have -and $have -ge $required) {
                Use-GoInstall -GoRoot $goRoot
                return
            }
        }
    }

    if ($env:OS -notlike '*Windows*') {
        throw "Go $required is required by go.mod; this host is not Windows and cannot run the official zip bootstrap"
    }

    $staging = Join-Path $ToolsRoot ".stage-$([guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Force -Path $staging | Out-Null
    try {
        $zipPath = Join-Path $staging $pin.archiveFilename
        Write-Host "downloading $($pin.archiveUrl)"
        Invoke-WebRequest -Uri $pin.archiveUrl -OutFile $zipPath -UseBasicParsing
        $digest = Get-Sha256Hex -Path $zipPath
        if ($digest -ne $pin.archiveSha256.ToLowerInvariant()) {
            throw "Go archive sha256 mismatch: got $digest, want $($pin.archiveSha256)"
        }
        Expand-Archive -LiteralPath $zipPath -DestinationPath $staging
        $extracted = Join-Path $staging "go"
        if (-not (Test-Path -LiteralPath (Join-Path $extracted "bin\go.exe") -PathType Leaf)) {
            throw "Go archive did not contain bin\go.exe"
        }
        if (Test-Path -LiteralPath $installDir) {
            Remove-Item -LiteralPath $installDir -Recurse -Force
        }
        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        Move-Item -LiteralPath $extracted -Destination $goRoot
        $markerTmp = Join-Path $installDir ".cliphub-go.json.tmp"
        $payload = @{
            version = $pin.version
            archiveSha256 = $pin.archiveSha256
            archiveUrl = $pin.archiveUrl
        } | ConvertTo-Json -Compress
        [System.IO.File]::WriteAllText($markerTmp, $payload)
        Move-Item -LiteralPath $markerTmp -Destination $marker -Force
    } finally {
        if (Test-Path -LiteralPath $staging) {
            Remove-Item -LiteralPath $staging -Recurse -Force
        }
    }

    Use-GoInstall -GoRoot $goRoot
}

if ($MyInvocation.InvocationName -ne '.') {
    Install-PinnedWindowsGo
}
