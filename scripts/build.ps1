# ClipHub Go runtime build. -RequireFaceitEmbed is the installer path: dist
# must fail rather than ship zv-orchestrator.exe without a FACEIT Data API key.
param(
    [switch]$RequireFaceitEmbed
)

$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "install-go-windows.ps1")

function Get-GoModVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$GoModPath
    )
    $line = Get-Content -LiteralPath $GoModPath | Where-Object { $_ -match '^go\s+(\d+\.\d+(?:\.\d+)?)\s*$' } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($line)) {
        throw "go.mod is missing a go version directive"
    }
    $raw = [regex]::Match($line, '^go\s+(\d+\.\d+(?:\.\d+)?)\s*$').Groups[1].Value
    if ($raw -notmatch '^\d+\.\d+\.\d+$') {
        $raw = "$raw.0"
    }
    return [version]$raw
}

function Get-InstalledGoVersion {
    $output = & go version
    if ($LASTEXITCODE -ne 0) {
        throw "go version failed with exit $LASTEXITCODE"
    }
    $match = [regex]::Match([string]$output, 'go version go(\d+\.\d+(?:\.\d+)?)')
    if (-not $match.Success) {
        throw "unrecognized go version output: $output"
    }
    $raw = $match.Groups[1].Value
    if ($raw -notmatch '^\d+\.\d+\.\d+$') {
        $raw = "$raw.0"
    }
    return [version]$raw
}

function Assert-GoToolchainMatchesModule {
    $goMod = Join-Path (Split-Path -Parent $PSScriptRoot) "go.mod"
    $required = Get-GoModVersion -GoModPath $goMod
    $pin = Read-PinnedWindowsGo
    if ($pin.version -ne "$required") {
        throw "scripts/go-windows.json version $($pin.version) does not match go.mod $required"
    }
    $installed = Get-InstalledGoVersion
    if ($installed -lt $required) {
        throw "Go $required or newer is required by go.mod; found $installed after Install-PinnedWindowsGo."
    }
}

function Get-FaceitEmbedKey {
    foreach ($scope in @("Process", "User", "Machine")) {
        $value = [Environment]::GetEnvironmentVariable("FACEIT_API_KEY", $scope)
        if (-not [string]::IsNullOrWhiteSpace($value)) {
            return $value.Trim()
        }
    }
    return ""
}

function Assert-FaceitKeyEmbedded {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Key
    )
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $hay = [System.Text.Encoding]::UTF8.GetString($bytes)
    if ($hay.IndexOf($Key, [StringComparison]::Ordinal) -lt 0) {
        throw "zv-orchestrator.exe is missing the embedded FACEIT Data API key"
    }
}

$commands = @(
    "zv",
    "zv-parser",
    "zv-demo-players",
    "zv-orchestrator",
    "zv-control-plane",
    "zv-recorder",
    "zv-composer",
    "zv-editor",
	"zv-stream",
    "zv-rhythm",
    "zv-analysis-viewer",
    "zv-tactical-data",
    "zv-tui"
)

$binDir = Join-Path (Resolve-Path ".").Path "bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
. (Join-Path $PSScriptRoot "build-publication.ps1")

$transactionID = [guid]::NewGuid().ToString("N")
$stagingDir = Join-Path $binDir ".build-$transactionID"
$backupDir = Join-Path $binDir ".backup-$transactionID"
$publicationLock = $null

try {
    Install-PinnedWindowsGo
    Assert-GoToolchainMatchesModule
    # Recover an interrupted publication before invoking any compiler, then keep
    # the same exclusive lease through the complete build and commit/rollback.
    $publicationLock = Enter-BuildPublicationLock -BinDir $binDir
    Recover-BuildPublication -BinDir $binDir -PublicationLock $publicationLock
    [void](New-Item -ItemType Directory -Path $stagingDir)
    $faceitEmbedKey = Get-FaceitEmbedKey
    if ($RequireFaceitEmbed -and $faceitEmbedKey -eq "") {
        throw "FACEIT_API_KEY is required to embed in zv-orchestrator.exe for the installer"
    }
    if ($faceitEmbedKey -ne "" -and $faceitEmbedKey -notmatch '^[A-Za-z0-9._-]+$') {
        throw "FACEIT_API_KEY contains unsupported characters for binary embedding"
    }

    foreach ($name in $commands) {
        $out = Join-Path $stagingDir "$name.exe"
        $pkg = "./cmd/$name"
        if ($name -eq "zv-orchestrator" -and $faceitEmbedKey -ne "") {
            Write-Host "go build -ldflags [faceit-embed] -o $out $pkg"
            & go build -ldflags "-X main.embeddedFaceitAPIKey=$faceitEmbedKey" -o $out $pkg
        } else {
            Write-Host "go build -o $out $pkg"
            & go build -o $out $pkg
        }
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $pkg"
        }
        $artifact = Get-Item -LiteralPath $out
        if ($artifact.PSIsContainer -or $artifact.Length -le 0) {
            throw "go build produced an invalid artifact for $pkg"
        }
        if ($name -eq "zv-orchestrator" -and $faceitEmbedKey -ne "") {
            Assert-FaceitKeyEmbedded -Path $out -Key $faceitEmbedKey
        }
    }

    # Publish only after the complete runtime set exists. The helper is a
    # testable transaction: every earlier target rolls back on failure, while an
    # incomplete rollback preserves the backup directory for manual recovery.
    Publish-BuildArtifacts -Names $commands -BinDir $binDir -StagingDir $stagingDir -BackupDir $backupDir -PublicationLock $publicationLock
} finally {
    try {
        # Both paths are GUID-named children of the resolved repository bin dir.
        if (Test-Path -LiteralPath $stagingDir) {
            Remove-Item -LiteralPath $stagingDir -Recurse -Force
        }
    } finally {
        if ($null -ne $publicationLock) {
            $publicationLock.Dispose()
        }
    }
}
