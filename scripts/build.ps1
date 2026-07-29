$ErrorActionPreference = "Stop"

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
    # Recover an interrupted publication before invoking any compiler, then keep
    # the same exclusive lease through the complete build and commit/rollback.
    $publicationLock = Enter-BuildPublicationLock -BinDir $binDir
    Recover-BuildPublication -BinDir $binDir -PublicationLock $publicationLock
    [void](New-Item -ItemType Directory -Path $stagingDir)
    foreach ($name in $commands) {
        $out = Join-Path $stagingDir "$name.exe"
        $pkg = "./cmd/$name"
        Write-Host "go build -o $out $pkg"
        & go build -o $out $pkg
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $pkg"
        }
        $artifact = Get-Item -LiteralPath $out
        if (-not $artifact.PSIsContainer -and $artifact.Length -gt 0) {
            continue
        }
        throw "go build produced an invalid artifact for $pkg"
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
