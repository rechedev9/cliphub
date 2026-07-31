param(
    [string]$Demo = "testdata\lavked-vs-tnc-m2-nuke.dem",
    [string]$TargetSteamID = "76561198148986856",
    [string]$BaseUrl = "",
    [ValidateRange(1, 65535)]
    [int]$OrchestratorPort = 18080,
    [int]$TimeoutSeconds = 1800,
    [string]$OutDir = "",
    [switch]$RequireFFprobe
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Fail {
    param([string]$Message)
    throw $Message
}

function Get-LoopbackListenerProcessID {
    param([int]$Port)
    try {
        $connections = @(Get-NetTCPConnection -State Listen -ErrorAction Stop)
    } catch {
        Fail "could not inspect loopback listeners: $($_.Exception.Message)"
    }
    $listeners = @(
        $connections |
            Where-Object { $_.LocalPort -eq $Port } |
            Where-Object { $_.LocalAddress -eq "127.0.0.1" -or $_.LocalAddress -eq "0.0.0.0" } |
            Select-Object -ExpandProperty OwningProcess -Unique
    )
    if ($listeners.Count -eq 0) {
        return 0
    }
    if ($listeners.Count -ne 1) {
        Fail "multiple processes listen on loopback port ${Port}: $($listeners -join ', ')"
    }
    return [int]$listeners[0]
}

function Test-ProcessBelongsToLaunch {
    param(
        [int]$CandidateProcessID,
        [int]$LaunchProcessID,
        [DateTime]$LaunchStartedAt
    )

    if ($CandidateProcessID -le 0 -or $LaunchProcessID -le 0) {
        return $false
    }
    $currentProcessID = $CandidateProcessID
    $descendantStartedAt = $null
    for ($depth = 0; $depth -lt 128 -and $currentProcessID -gt 0; $depth++) {
        if ($currentProcessID -eq $LaunchProcessID) {
            return $null -eq $descendantStartedAt -or
                $descendantStartedAt -ge $LaunchStartedAt
        }
        try {
            $process = Get-CimInstance Win32_Process `
                -Filter ("ProcessId = {0}" -f $currentProcessID) `
                -ErrorAction Stop
        } catch {
            Fail "could not inspect ancestry for process ${currentProcessID}: $($_.Exception.Message)"
        }
        if ($null -eq $process) {
            return $false
        }
        $currentStartedAt = ([DateTime]$process.CreationDate).ToUniversalTime()
        if ($null -ne $descendantStartedAt -and
            $currentStartedAt -gt $descendantStartedAt) {
            # ParentProcessId can outlive its process. Reject a newer process
            # that merely reused an ancestor's numeric PID.
            return $false
        }
        $descendantStartedAt = $currentStartedAt
        $currentProcessID = [int]$process.ParentProcessId
    }
    return $false
}

function Assert-OwnedListener {
    param(
        [Diagnostics.Process]$Server,
        [Diagnostics.Process]$Launch,
        [DateTime]$LaunchStartedAt,
        [int]$Port,
        [string]$Context
    )

    if ($Server.HasExited) {
        Fail "${Context}: owned listener process $($Server.Id) exited"
    }
    $listenerProcessID = Get-LoopbackListenerProcessID -Port $Port
    if ($listenerProcessID -ne $Server.Id) {
        Fail "${Context}: loopback port $Port is owned by process $listenerProcessID, expected $($Server.Id)"
    }
    if (-not (Test-ProcessBelongsToLaunch `
        -CandidateProcessID $Server.Id `
        -LaunchProcessID $Launch.Id `
        -LaunchStartedAt $LaunchStartedAt)) {
        Fail "${Context}: listener process $($Server.Id) is not descended from launch process $($Launch.Id)"
    }
}

function New-SmokeCapability {
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) }
    finally { $rng.Dispose() }
    return ([BitConverter]::ToString($bytes)).Replace("-", "").ToLowerInvariant()
}

function Invoke-Curl {
    param([string[]]$Arguments, [string]$Description)
    $headerName = "X-TickCut-" + "Token"
    $curlConfig = 'header = "{0}: {1}"' -f $headerName, $Capability
    # Windows PowerShell otherwise writes a UTF-16/BOM native pipeline, which
    # makes curl parse the first directive as "﻿header". Keep the capability on
    # stdin, but force BOM-free UTF-8 for this one native-process handoff.
    $previousOutputEncoding = $OutputEncoding
    $OutputEncoding = New-Object System.Text.UTF8Encoding($false)
    try {
        $output = $curlConfig | & curl.exe --config - @Arguments 2>&1
    } finally {
        $OutputEncoding = $previousOutputEncoding
    }
    if ($LASTEXITCODE -ne 0) {
        Fail "$Description failed: $($output -join "`n")"
    }
    return ($output -join "`n")
}

function Get-Job {
    param([string]$JobID)
    $raw = Invoke-Curl -Description "GET /api/jobs/$JobID" -Arguments @(
        "-fsS",
        "$BaseUrl/api/jobs/$JobID"
    )
    return ($raw | ConvertFrom-Json)
}

function Wait-JobStatus {
    param(
        [string]$JobID,
        [string[]]$Desired,
        [string]$Phase
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    $lastStatus = ""

    while ((Get-Date) -lt $deadline) {
        $job = Get-Job -JobID $JobID
        $status = [string]$job.status
        if ($status -ne $lastStatus) {
            Write-Host ("{0}: status={1}" -f $Phase, $status)
            $lastStatus = $status
        }
        if ($status -eq "failed") {
            Fail "$Phase failed: $($job.failure_reason)"
        }
        if ($Desired -contains $status) {
            $stopwatch.Stop()
            return [pscustomobject]@{
                Job = $job
                Seconds = [math]::Round($stopwatch.Elapsed.TotalSeconds, 2)
            }
        }
        Start-Sleep -Seconds 2
    }

    Fail "Timed out waiting for $Phase to reach $($Desired -join "/") after $TimeoutSeconds seconds."
}

function Invoke-JobPost {
    param([string]$JobID, [string]$Action)
    [void](Invoke-Curl -Description "POST /api/jobs/$JobID/$Action" -Arguments @(
        "-fsS",
        "-X", "POST",
        "$BaseUrl/api/jobs/$JobID/$Action"
    ))
}

function Convert-Ratio {
    param([string]$Ratio)
    $parts = $Ratio -split "/"
    if ($parts.Count -ne 2 -or [double]$parts[1] -eq 0) {
        return 0.0
    }
    return ([double]$parts[0] / [double]$parts[1])
}

function Test-FinalWithFFprobe {
    param([string]$Path)

    $ffprobe = [Environment]::GetEnvironmentVariable("ZV_FFPROBE_PATH")
    if ([string]::IsNullOrWhiteSpace($ffprobe)) {
        $cmd = Get-Command ffprobe -ErrorAction SilentlyContinue
        if ($cmd) {
            $ffprobe = $cmd.Source
        }
    }
    if ([string]::IsNullOrWhiteSpace($ffprobe) -or -not (Test-Path -LiteralPath $ffprobe)) {
        if ($RequireFFprobe) {
            Fail "ffprobe not found. Set ZV_FFPROBE_PATH or add ffprobe to PATH."
        }
        Write-Warning "ffprobe not found; skipping codec/fps/audio verification."
        return
    }

    $raw = & $ffprobe -v error -print_format json -show_streams $Path 2>&1
    if ($LASTEXITCODE -ne 0) {
        Fail "ffprobe failed: $($raw -join "`n")"
    }
    $probe = (($raw -join "`n") | ConvertFrom-Json)
    $video = @($probe.streams | Where-Object { $_.codec_type -eq "video" } | Select-Object -First 1)
    $audio = @($probe.streams | Where-Object { $_.codec_type -eq "audio" } | Select-Object -First 1)
    if ($video.Count -eq 0) {
        Fail "ffprobe did not find a video stream."
    }
    if ($audio.Count -eq 0) {
        Fail "ffprobe did not find an audio stream."
    }

    $fps = Convert-Ratio -Ratio ([string]$video[0].avg_frame_rate)
    if ($fps -eq 0) {
        $fps = Convert-Ratio -Ratio ([string]$video[0].r_frame_rate)
    }
    if ($video[0].codec_name -ne "h264") {
        Fail "Expected H.264 video, got $($video[0].codec_name)."
    }
    if ([int]$video[0].width -ne 1920 -or [int]$video[0].height -ne 1080) {
        Fail "Expected 1920x1080 video, got $($video[0].width)x$($video[0].height)."
    }
    if ([math]::Abs($fps - 60.0) -gt 0.2) {
        Fail "Expected 60fps video, got $fps."
    }
    Write-Host ("ffprobe: video=h264 {0}x{1} {2:n2}fps audio={3}" -f $video[0].width, $video[0].height, $fps, $audio[0].codec_name)
}

if (-not (Get-Command curl.exe -ErrorAction SilentlyContinue)) {
    Fail "curl.exe is required."
}

$ownedOrchestrator = $null
$ownedOrchestratorStartedAt = $null
$ownedServer = $null
$ownedDataDir = ""
$configuredBaseUrl = $BaseUrl
if ([string]::IsNullOrWhiteSpace($configuredBaseUrl)) {
    $configuredBaseUrl = [Environment]::GetEnvironmentVariable("ZV_BASE_URL")
}
try {
if ([string]::IsNullOrWhiteSpace($configuredBaseUrl)) {
    # The default smoke path owns an isolated orchestrator and its capability.
    # The secret lives only in this process and the child environment; it never
    # appears in argv, logs, or a persistent file.
    $Capability = New-SmokeCapability
    $BaseUrl = "http://127.0.0.1:$OrchestratorPort"
    $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
    $zvPath = Join-Path $repoRoot "bin\zv.exe"
    if (-not (Test-Path -LiteralPath $zvPath)) {
        Fail "missing $zvPath - build first with .\scripts\build.ps1"
    }
    $existingListenerPID = Get-LoopbackListenerProcessID -Port $OrchestratorPort
    if ($existingListenerPID -gt 0) {
        Fail "smoke port $OrchestratorPort is already owned by process $existingListenerPID"
    }
    $ownedDataDir = Join-Path ([IO.Path]::GetTempPath()) ("tickcut-smoke-" + [guid]::NewGuid().ToString("N"))
    [void](New-Item -ItemType Directory -Path $ownedDataDir)
    $ownedNames = @("ZV_DATABASE_URL", "ZV_DATA_DIR", "ZV_HTTP_ADDR", "ZV_MUTATION_TOKEN")
    $ownedOriginal = @{}
    foreach ($name in $ownedNames) {
        $ownedOriginal[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
    }
    try {
        $env:ZV_DATABASE_URL = "memory"
        $env:ZV_DATA_DIR = $ownedDataDir
        $env:ZV_HTTP_ADDR = "127.0.0.1:$OrchestratorPort"
        $env:ZV_MUTATION_TOKEN = $Capability
        # Canonical launch contract: bin\zv serve. Teardown resolves the actual
        # listener because stopping the wrapper alone does not prove its child
        # has released the port.
        $ownedOrchestrator = Start-Process -FilePath $zvPath -ArgumentList @("serve") -PassThru -WindowStyle Hidden
        # Retaining the original process object handle prevents PID reuse from
        # turning later ancestry and teardown checks into checks of a new process.
        [void]$ownedOrchestrator.Handle
        $ownedOrchestratorStartedAt = $ownedOrchestrator.StartTime.ToUniversalTime()
    } finally {
        foreach ($name in $ownedNames) {
            [Environment]::SetEnvironmentVariable($name, $ownedOriginal[$name], "Process")
        }
    }
    Write-Host "Started isolated smoke orchestrator at $BaseUrl"
} else {
    $BaseUrl = $configuredBaseUrl
    $Capability = [Console]::In.ReadLine()
    if ($Capability -notmatch '^[0-9a-f]{64}$') {
        Fail "Pipe the external orchestrator session capability to stdin (64 lowercase hex characters)."
    }
}
$BaseUrl = $BaseUrl.TrimEnd("/")

if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $OutDir = Join-Path (Get-Location) "data\smoke"
}
$DemoPath = (Resolve-Path -LiteralPath $Demo).Path
[void](New-Item -ItemType Directory -Force -Path $OutDir)

$healthStatus = ""
$foreignListenerPID = 0
$listenerInspectionFailure = ""
for ($attempt = 0; $attempt -lt 40; $attempt++) {
    if ($ownedServer -and $ownedServer.HasExited) {
        $listenerInspectionFailure = "owned listener process $($ownedServer.Id) exited during startup"
        break
    }
    if ($ownedOrchestrator) {
        try {
            $listenerPID = Get-LoopbackListenerProcessID -Port $OrchestratorPort
            if ($ownedServer) {
                Assert-OwnedListener `
                    -Server $ownedServer `
                    -Launch $ownedOrchestrator `
                    -LaunchStartedAt $ownedOrchestratorStartedAt `
                    -Port $OrchestratorPort `
                    -Context "startup ownership check"
            } elseif ($listenerPID -gt 0) {
                if (-not (Test-ProcessBelongsToLaunch `
                    -CandidateProcessID $listenerPID `
                    -LaunchProcessID $ownedOrchestrator.Id `
                    -LaunchStartedAt $ownedOrchestratorStartedAt)) {
                    $foreignListenerPID = $listenerPID
                    break
                }
                $candidateServer = Get-Process -Id $listenerPID -ErrorAction Stop
                # Acquire the original process handle now. Teardown can then
                # distinguish this listener even if its wrapper exits or the
                # numeric PID is later reused.
                [void]$candidateServer.Handle
                Assert-OwnedListener `
                    -Server $candidateServer `
                    -Launch $ownedOrchestrator `
                    -LaunchStartedAt $ownedOrchestratorStartedAt `
                    -Port $OrchestratorPort `
                    -Context "listener adoption"
                $ownedServer = $candidateServer
            }
        } catch {
            $listenerInspectionFailure = $_.Exception.Message
            break
        }
    }
    $healthStatus = & curl.exe -sS -o NUL -w "%{http_code}" "$BaseUrl/healthz" 2>$null
    if ($LASTEXITCODE -eq 0 -and
        $healthStatus -eq "200" -and
        (-not $ownedOrchestrator -or ($ownedServer -and -not $ownedServer.HasExited))) {
        if ($ownedOrchestrator) {
            Assert-OwnedListener `
                -Server $ownedServer `
                -Launch $ownedOrchestrator `
                -LaunchStartedAt $ownedOrchestratorStartedAt `
                -Port $OrchestratorPort `
                -Context "health response ownership check"
        }
        break
    }
    Start-Sleep -Milliseconds 250
}
if (-not [string]::IsNullOrWhiteSpace($listenerInspectionFailure)) {
    Fail "could not prove ownership of the isolated smoke listener at ${BaseUrl}: $listenerInspectionFailure"
}
if ($foreignListenerPID -gt 0) {
    Fail "smoke port $OrchestratorPort became owned by unrelated process $foreignListenerPID"
}
if ($LASTEXITCODE -ne 0 -or
    $healthStatus -ne "200" -or
    ($ownedOrchestrator -and -not $ownedServer)) {
    $wrapperState = if ($ownedOrchestrator -and $ownedOrchestrator.HasExited) {
        " wrapper_exit_code=$($ownedOrchestrator.ExitCode)"
    } else {
        ""
    }
    Fail "Orchestrator listener did not become healthy at $BaseUrl (health status=$healthStatus).$wrapperState"
}

if ($ownedOrchestrator) {
    Assert-OwnedListener `
        -Server $ownedServer `
        -Launch $ownedOrchestrator `
        -LaunchStartedAt $ownedOrchestratorStartedAt `
        -Port $OrchestratorPort `
        -Context "capabilities request ownership check"
}
$capabilitiesRaw = Invoke-Curl -Description "GET /api/capabilities" -Arguments @("-fsS", "$BaseUrl/api/capabilities")
if ($ownedOrchestrator) {
    Assert-OwnedListener `
        -Server $ownedServer `
        -Launch $ownedOrchestrator `
        -LaunchStartedAt $ownedOrchestratorStartedAt `
        -Port $OrchestratorPort `
        -Context "capabilities response ownership check"
}
$capabilities = $capabilitiesRaw | ConvertFrom-Json
if (-not [bool]$capabilities.record.enabled -or -not [bool]$capabilities.compose.enabled) {
    Fail "The running Local Studio does not advertise both record and compose workers; inspect GET /api/capabilities before a real-media smoke run."
}

Write-Host "Uploading demo..."
$uploadWatch = [System.Diagnostics.Stopwatch]::StartNew()
$configJson = ('{{"target_steamid":"{0}"}}' -f $TargetSteamID)
$jobRaw = Invoke-Curl -Description "POST /api/jobs" -Arguments @(
    "-fsS",
    "-X", "POST",
    "$BaseUrl/api/jobs",
    "-F", "demo=@$DemoPath",
    "-F", "config=$configJson"
)
$uploadWatch.Stop()
$job = ($jobRaw | ConvertFrom-Json)
$jobID = [string]$job.id
Write-Host ("job_id={0}" -f $jobID)

$parsed = Wait-JobStatus -JobID $jobID -Desired @("parsed") -Phase "parse"

Invoke-JobPost -JobID $jobID -Action "record"
$recorded = Wait-JobStatus -JobID $jobID -Desired @("recorded") -Phase "record"

Invoke-JobPost -JobID $jobID -Action "record"
Start-Sleep -Seconds 2
$recordRetry = Get-Job -JobID $jobID
if ([string]$recordRetry.status -ne "recorded") {
    $recordedRetry = Wait-JobStatus -JobID $jobID -Desired @("recorded") -Phase "record-retry"
    $recordRetrySeconds = $recordedRetry.Seconds
} else {
    $recordRetrySeconds = 0
}

Invoke-JobPost -JobID $jobID -Action "compose"
$composed = Wait-JobStatus -JobID $jobID -Desired @("composed") -Phase "compose"

Invoke-JobPost -JobID $jobID -Action "compose"
Start-Sleep -Seconds 2
$composeRetry = Get-Job -JobID $jobID
if ([string]$composeRetry.status -ne "composed") {
    $composedRetry = Wait-JobStatus -JobID $jobID -Desired @("composed") -Phase "compose-retry"
    $composeRetrySeconds = $composedRetry.Seconds
} else {
    $composeRetrySeconds = 0
}

$finalPath = Join-Path $OutDir "$jobID-final.mp4"
[void](Invoke-Curl -Description "GET /api/jobs/$jobID/final" -Arguments @(
    "-fsS",
    "-L",
    "-o", $finalPath,
    "$BaseUrl/api/jobs/$jobID/final"
))
$finalSize = (Get-Item -LiteralPath $finalPath).Length
if ($finalSize -le 0) {
    Fail "Downloaded final MP4 is empty: $finalPath"
}

Test-FinalWithFFprobe -Path $finalPath

Write-Host ("upload_seconds={0:n2}" -f $uploadWatch.Elapsed.TotalSeconds)
Write-Host ("parse_seconds={0:n2}" -f $parsed.Seconds)
Write-Host ("record_seconds={0:n2}" -f $recorded.Seconds)
Write-Host ("record_retry_seconds={0:n2}" -f $recordRetrySeconds)
Write-Host ("compose_seconds={0:n2}" -f $composed.Seconds)
Write-Host ("compose_retry_seconds={0:n2}" -f $composeRetrySeconds)
Write-Host ("final_path={0}" -f $finalPath)
Write-Host ("final_size_bytes={0}" -f $finalSize)
}
finally {
    if ($ownedServer -and -not $ownedServer.HasExited) {
        try {
            if (Test-ProcessBelongsToLaunch `
                -CandidateProcessID $ownedServer.Id `
                -LaunchProcessID $ownedOrchestrator.Id `
                -LaunchStartedAt $ownedOrchestratorStartedAt) {
                & taskkill.exe /PID $ownedServer.Id /T /F *> $null
                $ownedServer.WaitForExit()
            } else {
                Write-Warning "refusing to terminate listener process $($ownedServer.Id): it no longer belongs to launch process $($ownedOrchestrator.Id)"
            }
        } catch {
            Write-Warning "refusing to terminate listener process $($ownedServer.Id): ancestry inspection failed: $($_.Exception.Message)"
        }
    }
    if ($ownedOrchestrator -and -not $ownedOrchestrator.HasExited) {
        if (-not $ownedServer -or $ownedOrchestrator.Id -ne $ownedServer.Id) {
            & taskkill.exe /PID $ownedOrchestrator.Id /T /F *> $null
        }
        $ownedOrchestrator.WaitForExit()
    }
    if (-not [string]::IsNullOrWhiteSpace($ownedDataDir) -and (Test-Path -LiteralPath $ownedDataDir)) {
        $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd("\") + "\"
        $resolvedOwnedData = [IO.Path]::GetFullPath($ownedDataDir)
        if (-not $resolvedOwnedData.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
            Fail "refusing to remove smoke data outside the system temp directory: $resolvedOwnedData"
        }
        for ($attempt = 0; $attempt -lt 20 -and (Test-Path -LiteralPath $resolvedOwnedData); $attempt++) {
            Remove-Item -LiteralPath $resolvedOwnedData -Recurse -Force -ErrorAction SilentlyContinue
            if (Test-Path -LiteralPath $resolvedOwnedData) {
                Start-Sleep -Milliseconds 100
            }
        }
        if (Test-Path -LiteralPath $resolvedOwnedData) {
            Fail "could not remove isolated smoke data: $resolvedOwnedData"
        }
    }
}
