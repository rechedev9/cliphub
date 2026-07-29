# FragForge Local Studio
#
# One command to run the whole product on the user's own Windows + GPU PC: the
# orchestrator (parse + HLAE/CS2 capture + render) and the web UI, wired so the
# browser flow (upload -> pick player -> pick kills -> create reel) actually
# drives local capture. Everything runs on this machine.
#
# What it does:
#   1. Starts `zv serve` with an on-disk SQLite job database and inline queue
#      (no external database or queue service). The orchestrator auto-detects
#      HLAE, CS2, and zv-recorder on startup, so capture works without setting
#      any tool-path env vars.
#   2. Starts the Next.js web UI, whose /api/demos/* routes proxy the full
#      pipeline to the local orchestrator.
#   3. Opens the browser at the upload page.
#
# Ctrl+C stops the web UI and then the orchestrator. Pass explicit loopback
# ports (and a separate data directory) to run an isolated second instance:
# .\scripts\local-studio.ps1 -OrchestratorPort 18080 -WebPort 13000 -DataDir C:\temp\fragforge-qa
# Browser automation should also pass -NoBrowser, provide its own
# FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY, and navigate its browser session to
# /bootstrap#<capability>. The fragment is removed before the normal POST.
#
# Prerequisites:
#   - Build the binaries first:  .\scripts\build.ps1   (produces .\bin\zv.exe)
#   - Node.js + pnpm 11.9.0. If node_modules is missing, the script installs
#     exactly the dependency graph in pnpm-lock.yaml.
#   - CS2 + the latest official HLAE installed under C:\HLAE-<version>\HLAE.exe. Capture needs them;
#     without them the app still runs the analyze flow and the Capture card tells
#     you what is missing.

[CmdletBinding()]
param(
    [ValidateRange(1, 65535)]
    [int]$OrchestratorPort = 8080,
    [ValidateRange(1, 65535)]
    [int]$WebPort = 3000,
    [string]$DataDir = "",
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"
if ($OrchestratorPort -eq $WebPort) {
    throw "-OrchestratorPort and -WebPort must be different"
}

# Keep every service descendant inside a kill-on-close Windows Job Object.
# `finally` handles ordinary Ctrl+C/error exits; the OS job also handles an
# externally terminated launcher, where PowerShell has no chance to run it.
. (Join-Path $PSScriptRoot "local-process-job.ps1")

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$binZv = Join-Path $repoRoot "bin\zv.exe"
$binOrchestrator = Join-Path $repoRoot "bin\zv-orchestrator.exe"
$webDir = Join-Path $repoRoot "web"
$dataDir = if ([string]::IsNullOrWhiteSpace($DataDir)) {
    Join-Path $repoRoot "data"
} else {
    [System.IO.Path]::GetFullPath($DataDir)
}

# Loopback only: the web server proxies to the orchestrator over localhost, and
# the browser only ever talks to the web server. Distinct process-local
# capabilities keep the Next proxy, the standalone bootstrap form, and the Go
# orchestrator from sharing one bearer secret.
$orchestratorUrl = "http://127.0.0.1:$OrchestratorPort"
$webUrl = "http://127.0.0.1:$WebPort"

function New-LocalCapability {
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) }
    finally { $rng.Dispose() }
    return ([BitConverter]::ToString($bytes)).Replace('-', '').ToLowerInvariant()
}

$mutatedEnvironmentNames = @(
    "ZV_DATABASE_URL",
    "ZV_DATA_DIR",
    "ZV_HTTP_ADDR",
    "ZV_MUTATION_TOKEN",
    "ORCHESTRATOR_URL",
    "ORCHESTRATOR_TOKEN",
    "FRAGFORGE_PROXY_MUTATION_CAPABILITY",
    "FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY",
    "PORT",
    "HOSTNAME"
)
$originalEnvironment = @{}
foreach ($name in $mutatedEnvironmentNames) {
    $originalEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}
$mutationCapability = if ([string]::IsNullOrWhiteSpace($originalEnvironment["ZV_MUTATION_TOKEN"])) {
    New-LocalCapability
} else {
    $originalEnvironment["ZV_MUTATION_TOKEN"]
}
if ($mutationCapability -notmatch '^[0-9a-f]{64}$') {
    throw "ZV_MUTATION_TOKEN must be 32 random bytes encoded as 64 lowercase hexadecimal characters"
}
$proxyCapability = if ([string]::IsNullOrWhiteSpace($originalEnvironment["FRAGFORGE_PROXY_MUTATION_CAPABILITY"])) {
    New-LocalCapability
} else {
    $originalEnvironment["FRAGFORGE_PROXY_MUTATION_CAPABILITY"]
}
$bootstrapCapability = if ([string]::IsNullOrWhiteSpace($originalEnvironment["FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY"])) {
    if ($NoBrowser) {
        throw "-NoBrowser requires FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY so the caller can complete the browser handshake"
    }
    New-LocalCapability
} else {
    $originalEnvironment["FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY"]
}
foreach ($capability in @($proxyCapability, $bootstrapCapability)) {
    if ($capability -notmatch '^[0-9a-f]{64}$') {
        throw "standalone proxy capabilities must be 32 random bytes encoded as 64 lowercase hexadecimal characters"
    }
}
if ((@($mutationCapability, $proxyCapability, $bootstrapCapability) | Select-Object -Unique).Count -ne 3) {
    throw "local Studio capabilities must be distinct"
}
foreach ($name in @(
    "ZV_MUTATION_TOKEN",
    "ORCHESTRATOR_TOKEN",
    "FRAGFORGE_PROXY_MUTATION_CAPABILITY",
    "FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY"
)) {
    [Environment]::SetEnvironmentVariable($name, $null, "Process")
}

$processJob = [IntPtr]::Zero
try {
$processJob = [FragForge.LocalProcessJob]::CreateKillOnClose()
if (-not (Test-Path $binZv)) {
    throw "missing $binZv - build the binaries first with .\scripts\build.ps1"
}
if (-not (Test-Path $binOrchestrator)) {
    throw "missing $binOrchestrator - build the binaries first with .\scripts\build.ps1"
}

if (-not (Test-Path (Join-Path $webDir "node_modules"))) {
    Write-Host "[local-studio] installing web dependencies (first run)..."
    Push-Location $webDir
    try { & pnpm install --frozen-lockfile; if ($LASTEXITCODE -ne 0) { throw "pnpm install failed" } }
    finally { Pop-Location }
}

New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

# Provision the music library into <repo>\data\music, the directory the
# orchestrator serves when ZV_MUSIC_DIR is unset. Best-effort: a machine that is
# offline (or already provisioned) still boots Local Studio, just with whatever
# tracks are present. sha256 mismatches are discarded so a truncated download
# never poisons the library.
$musicDir = Join-Path $dataDir "music"
$catalogPath = Join-Path $musicDir "catalog.json"
if (Test-Path $catalogPath) {
    Write-Host "[local-studio] provisioning music library (best-effort)..."
    try {
        $catalog = Get-Content $catalogPath -Raw | ConvertFrom-Json
        foreach ($track in $catalog.tracks) {
            if (-not $track.downloadUrl -or
                $track.id -notmatch '^[a-zA-Z0-9][a-zA-Z0-9_-]*$' -or
                $track.ext -notmatch '^[a-zA-Z0-9]+$') { continue }
            $trackPath = Join-Path $musicDir "$($track.id).$($track.ext)"
            if (-not $track.sha256 -or $track.sha256 -notmatch '^[a-fA-F0-9]{64}$') {
                Remove-Item -LiteralPath $trackPath -Force -ErrorAction SilentlyContinue
                Write-Host "[local-studio]   discarded $($track.id): missing or invalid catalog sha256"
                continue
            }
            if (Test-Path $trackPath) {
                try {
                    $cachedDigest = (Get-FileHash -Path $trackPath -Algorithm SHA256).Hash.ToLowerInvariant()
                    if ($cachedDigest -eq $track.sha256.ToLowerInvariant()) {
                        Write-Host "[local-studio]   verified $($track.id).$($track.ext)"
                        continue
                    }
                    Remove-Item -LiteralPath $trackPath -Force
                    Write-Host "[local-studio]   discarded $($track.id).$($track.ext) (sha256 mismatch)"
                } catch {
                    Remove-Item -LiteralPath $trackPath -Force -ErrorAction SilentlyContinue
                    Write-Host "[local-studio]   discarded $($track.id).$($track.ext) (could not verify sha256)"
                }
            }
            $tempPath = $null
            try {
                $tempPath = Join-Path $musicDir ".music-$($track.id)-$([guid]::NewGuid().ToString('N')).tmp"
                Invoke-WebRequest -Uri $track.downloadUrl -OutFile $tempPath -UseBasicParsing -TimeoutSec 60
                $digest = (Get-FileHash -Path $tempPath -Algorithm SHA256).Hash.ToLowerInvariant()
                if ($digest -ne $track.sha256.ToLowerInvariant()) {
                    Remove-Item -LiteralPath $tempPath -Force -ErrorAction SilentlyContinue
                    Write-Host "[local-studio]   discarded $($track.id) (sha256 mismatch)"
                } else {
                    Move-Item -LiteralPath $tempPath -Destination $trackPath -Force
                    Write-Host "[local-studio]   downloaded $($track.id).$($track.ext)"
                }
            } catch {
                if ($tempPath -and (Test-Path $tempPath)) { Remove-Item -LiteralPath $tempPath -Force -ErrorAction SilentlyContinue }
                Write-Host "[local-studio]   skipped $($track.id): $($_.Exception.Message)"
            }
        }
    } catch {
        Write-Host "[local-studio] music provisioning skipped: $($_.Exception.Message)"
    }
}

Write-Host "[local-studio] starting orchestrator (SQLite jobs, capture auto-detected)..."
# Set in the session so the orchestrator child inherits them (works on both
# Windows PowerShell 5.1 and PowerShell 7; Start-Process -Environment is 7.4+).
# sqlite = on-disk job repo (<dataDir>/jobs.db) + inline queue, with no external
# services, so jobs survive a restart.
$env:ZV_DATABASE_URL = "sqlite"
$env:ZV_DATA_DIR = $dataDir
$env:ZV_HTTP_ADDR = "127.0.0.1:$OrchestratorPort"
$env:ZV_MUTATION_TOKEN = $mutationCapability
try {
    # Own the actual service process. Starting through `zv serve` owns only the
    # short-lived delegate wrapper, so teardown can otherwise leave the
    # orchestrator grandchild listening after Local Studio exits.
    $orchestrator = [FragForge.LocalProcessJob]::StartInJob(
        $processJob,
        $binOrchestrator,
        "",
        $repoRoot
    )
} finally {
    [Environment]::SetEnvironmentVariable("ZV_MUTATION_TOKEN", $null, "Process")
}

try {
    # Wait for the orchestrator to answer /healthz before starting the web UI, so
    # the first upload does not race a not-yet-listening backend.
    Write-Host "[local-studio] waiting for orchestrator health..."
    $healthy = $false
    for ($i = 0; $i -lt 40; $i++) {
        if ($orchestrator.HasExited) { throw "orchestrator exited early (code $($orchestrator.ExitCode))" }
        try {
            $res = Invoke-WebRequest -Uri "$orchestratorUrl/healthz" -UseBasicParsing -TimeoutSec 2
            if ($res.StatusCode -eq 200) { $healthy = $true; break }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $healthy) { throw "orchestrator did not become healthy at $orchestratorUrl" }
    Write-Host "[local-studio] orchestrator healthy at $orchestratorUrl"

    # The /api/demos/* routes proxy the whole pipeline to the local
    # orchestrator; these values are read server-side by the Next.js server.
    $env:ORCHESTRATOR_URL = $orchestratorUrl
    $env:ORCHESTRATOR_TOKEN = $mutationCapability
    $env:FRAGFORGE_PROXY_MUTATION_CAPABILITY = $proxyCapability
    $env:FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY = $bootstrapCapability
    $env:PORT = "$WebPort"
    # Next's development server otherwise binds to :: and advertises a LAN URL.
    # Studio is a local control plane, so both services must stay on loopback.
    $env:HOSTNAME = "127.0.0.1"

    if ($NoBrowser) {
        Write-Host "[local-studio] browser launch disabled; navigate the owned browser to $webUrl/bootstrap#<the provided bootstrap capability>"
    } else {
        Write-Host "[local-studio] standalone browser will authorize itself with a one-launch local capability"
    }
    Write-Host "[local-studio] starting web UI (Ctrl+C to stop everything)..."
    $nodeCommand = (Get-Command node.exe -ErrorAction Stop).Source
    $nextCommand = Join-Path $webDir "node_modules\next\dist\bin\next"
    # Windows PowerShell 5 flattens ArgumentList into one command line, so an
    # entrypoint beneath a repository path containing spaces must retain quotes.
    $quotedNextCommand = '"' + $nextCommand + '"'
    $webArguments = "$quotedNextCommand dev --hostname 127.0.0.1 --port $WebPort"
    $webProcess = [FragForge.LocalProcessJob]::StartInJob(
        $processJob,
        $nodeCommand,
        $webArguments,
        $webDir
    )
    # Next inherited its server-only capabilities at spawn. Remove them from
    # the launcher before any browser process is created; finally restores the
    # caller's exact prior environment.
    $env:ORCHESTRATOR_TOKEN = $null
    $env:FRAGFORGE_PROXY_MUTATION_CAPABILITY = $null
    $env:FRAGFORGE_PROXY_BOOTSTRAP_CAPABILITY = $null

    Write-Host "[local-studio] waiting for web UI..."
    $webReady = $false
    for ($i = 0; $i -lt 120; $i++) {
        if ($webProcess.HasExited) {
            throw "web UI exited early (code $($webProcess.ExitCode))"
        }
        try {
            $response = Invoke-WebRequest -Uri "$webUrl/bootstrap" -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                $webReady = $true
                break
            }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    if (-not $webReady) {
        throw "web UI did not become healthy at $webUrl"
    }
    Write-Host "[local-studio] web UI healthy at $webUrl"
    if (-not $NoBrowser) {
        # URL fragments are not sent to the web server. The bootstrap page
        # removes this one-launch value from history before posting it and
        # receiving the HttpOnly mutation cookie.
        Start-Process "$webUrl/bootstrap#$bootstrapCapability"
    }

    while (-not $webProcess.HasExited) {
        Start-Sleep -Milliseconds 500
    }
    if ($webProcess.ExitCode -ne 0) {
        throw "web UI exited (code $($webProcess.ExitCode))"
    }
}
finally {
    if ($orchestrator -and -not $orchestrator.HasExited) {
        Write-Host "[local-studio] stopping orchestrator..."
        Stop-Process -Id $orchestrator.Id -Force -ErrorAction SilentlyContinue
    }
}
}
finally {
    if ($processJob -ne [IntPtr]::Zero) {
        [FragForge.LocalProcessJob]::Close($processJob)
    }
    foreach ($name in $mutatedEnvironmentNames) {
        [Environment]::SetEnvironmentVariable($name, $originalEnvironment[$name], "Process")
    }
}
