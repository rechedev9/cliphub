# ClipHub Go runtime build. -RequireFaceitEmbed is the installer path: dist
# must fail rather than ship zv-orchestrator.exe without a FACEIT Data API key.
param(
    [switch]$RequireFaceitEmbed,
    # Build the TeamSpeak plugin even when gcc is not found, so a missing gcc fails the build.
    # -RequireFaceitEmbed implies it.
    [switch]$TeamSpeakPlugin
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

# The library retry after an upgrade keys on vcs.revision; without it every build reads as "devel".
# Returns the stamped revision and whether vcs.modified=true.
function Assert-VcsRevisionStamped {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )
    $output = & go version -m $Path
    if ($LASTEXITCODE -ne 0) {
        throw "go version -m failed for $Path with exit $LASTEXITCODE"
    }
    $settings = @($output | ForEach-Object { [string]$_ })
    $revision = $settings | ForEach-Object { [regex]::Match($_, '^\s*build\s+vcs\.revision=(\S+)\s*$') } |
        Where-Object { $_.Success } | Select-Object -First 1
    if ($null -eq $revision) {
        throw "$(Split-Path -Leaf $Path) carries no vcs.revision. Build from a git checkout with git on PATH and without -buildvcs=false in GOFLAGS."
    }
    return [pscustomobject]@{
        Revision = $revision.Groups[1].Value
        Modified = [bool]($settings | Where-Object { $_ -match '^\s*build\s+vcs\.modified=true\s*$' })
    }
}

$commands = @(
    "zv",
    "zv-parser",
    "zv-demo-players",
    "zv-orchestrator",
    "zv-recorder",
    "zv-composer",
    "zv-editor",
	"zv-stream",
    "zv-rhythm",
    "zv-analysis-viewer",
    "zv-tactical-data",
    "zv-hud-designs",
    "zv-tui",
    "zv-mcp"
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
    # The TeamSpeak plugin needs gcc; the installer path must not ship without it, so fail before any build.
    $hasGcc = [bool](Get-Command gcc -ErrorAction SilentlyContinue)
    if (($TeamSpeakPlugin -or $RequireFaceitEmbed) -and -not $hasGcc) {
        throw "gcc (MinGW-w64) is not on PATH, so the TeamSpeak 3 plugin cannot be built and a release must not ship without it. Install MinGW-w64 and retry."
    }
    $renderManifest = Join-Path (Resolve-Path ".").Path "tools\render\Cargo.toml"
    $hasCargo = [bool](Get-Command cargo -ErrorAction SilentlyContinue)
    if ($RequireFaceitEmbed -and -not $hasCargo) {
        throw "cargo (Rust toolchain) is not on PATH, so cliphub-render.exe cannot be built and a release must not ship without it. Install Rust (rustup) and retry."
    }
    $faceitEmbedKey = Get-FaceitEmbedKey
    if ($RequireFaceitEmbed -and $faceitEmbedKey -eq "") {
        throw "FACEIT_API_KEY is required to embed in zv-orchestrator.exe for the installer"
    }
    if ($faceitEmbedKey -ne "" -and $faceitEmbedKey -notmatch '^[A-Za-z0-9._-]+$') {
        throw "FACEIT_API_KEY contains unsupported characters for binary embedding"
    }

    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        throw "git is not on PATH, so go build cannot stamp vcs.revision and the library retry after an upgrade would compare devel with devel. Install Git and retry."
    }
    $headRevision = [string](& git rev-parse HEAD)
    if ($LASTEXITCODE -ne 0) {
        throw "git rev-parse HEAD failed with exit $LASTEXITCODE"
    }
    $isGitWorktree = Test-Path -LiteralPath (Join-Path (Resolve-Path ".").Path ".git") -PathType Leaf
    $modifiedTree = $false
    $foreignRevision = ""
    $worktreeSkippedStamp = $false
    foreach ($name in $commands) {
        $out = Join-Path $stagingDir "$name.exe"
        $pkg = "./cmd/$name"
        # -s -w drops the symbol table and DWARF (about a quarter of each binary);
        # panic stack traces still resolve through the runtime pclntab.
        $ldflags = "-s -w"
        $shownFlags = $ldflags
        if ($name -eq "zv-orchestrator" -and $faceitEmbedKey -ne "") {
            $ldflags += " -X main.embeddedFaceitAPIKey=$faceitEmbedKey"
            $shownFlags += " faceit-embed"
        }
        Write-Host "go build -ldflags [$shownFlags] -o $out $pkg"
        & go build -ldflags $ldflags -o $out $pkg
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
        try {
            $stamp = Assert-VcsRevisionStamped -Path $out
            if ($stamp.Modified) {
                $modifiedTree = $true
            }
            if ($stamp.Revision -ne $headRevision.Trim()) {
                $foreignRevision = $stamp.Revision
            }
        } catch {
            if ($RequireFaceitEmbed -or -not $isGitWorktree) {
                throw
            }
            $worktreeSkippedStamp = $true
        }
    }
    # Go stamps the enclosing checkout when the repo is a worktree nested in another one.
    if ($foreignRevision -ne "") {
        $message = "The binaries carry vcs.revision $foreignRevision, not HEAD $($headRevision.Trim()). Build from a checkout that is not nested in another git repo."
        if ($RequireFaceitEmbed) {
            throw $message
        }
        Write-Host $message -ForegroundColor Yellow
    }
    if ($worktreeSkippedStamp) {
        Write-Host "Skipped vcs.revision check: git worktree checkout (.git file) is not stamped by Go -buildvcs. Release builds (-RequireFaceitEmbed) still require a full checkout." -ForegroundColor Yellow
    }
    if ($modifiedTree) {
        Write-Host "The binaries carry vcs.modified=true: uncommitted changes keep the commit's vcs.revision, so the library retry after an upgrade does not treat them as a new build." -ForegroundColor Yellow
    }

    $publishNames = [System.Collections.Generic.List[string]]::new()
    foreach ($name in $commands) {
        $publishNames.Add($name)
    }

    if ($hasCargo) {
        Write-Host "cargo build --release --manifest-path tools/render/Cargo.toml"
        & cargo build --release --manifest-path $renderManifest
        if ($LASTEXITCODE -ne 0) {
            throw "cargo build failed for tools/render"
        }
        $cargoTargetDir = if (-not [string]::IsNullOrWhiteSpace($env:CARGO_TARGET_DIR)) {
            $env:CARGO_TARGET_DIR
        } else {
            Join-Path (Split-Path -Parent $renderManifest) "target"
        }
        $renderBuilt = Join-Path $cargoTargetDir "release\cliphub-render.exe"
        if (-not (Test-Path -LiteralPath $renderBuilt -PathType Leaf)) {
            throw "cargo build did not produce $renderBuilt"
        }
        $renderStaged = Join-Path $stagingDir "cliphub-render.exe"
        Copy-Item -LiteralPath $renderBuilt -Destination $renderStaged -Force
        $renderArtifact = Get-Item -LiteralPath $renderStaged
        if ($renderArtifact.PSIsContainer -or $renderArtifact.Length -le 0) {
            throw "cargo build produced an invalid artifact for cliphub-render"
        }
        $publishNames.Add("cliphub-render")
    } else {
        Write-Host "GPU render engine skipped: cargo (Rust toolchain) is not on PATH. Install Rust (rustup) to build bin\cliphub-render.exe." -ForegroundColor Yellow
    }

    # Publish only after the complete runtime set exists. The helper is a
    # testable transaction: every earlier target rolls back on failure, while an
    # incomplete rollback preserves the backup directory for manual recovery.
    Publish-BuildArtifacts -Names @($publishNames) -BinDir $binDir -StagingDir $stagingDir -BackupDir $backupDir -PublicationLock $publicationLock

    # The TeamSpeak 3 plugin package ships in the installer beside the Go runtimes; it needs MinGW-w64 gcc.
    if ($hasGcc) {
        & (Join-Path $PSScriptRoot "build-teamspeak-plugin.ps1")
        if ($LASTEXITCODE -ne 0) {
            throw "build-teamspeak-plugin.ps1 failed with exit $LASTEXITCODE"
        }
    } else {
        Write-Host "TeamSpeak plugin skipped: gcc (MinGW-w64) is not on PATH. Install it or pass -TeamSpeakPlugin to build bin\cliphub_teamspeak.ts3_plugin." -ForegroundColor Yellow
    }
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
