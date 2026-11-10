# Builds bin\cliphub_teamspeak.ts3_plugin (64-bit TeamSpeak 3 plugin) with MinGW-w64 gcc.
# The TS3 plugin SDK headers are "all rights reserved" upstream, so they are fetched from a
# pinned commit with a SHA-256 check into plugins\teamspeak\build\sdk instead of being vendored.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$pluginDir = Join-Path $root 'plugins\teamspeak'
$build = Join-Path $pluginDir 'build'
$sdk = Join-Path $build 'sdk'
$out = Join-Path $root 'bin\cliphub_teamspeak.ts3_plugin'

$sdkCommit = '4aa90a53aa150cbf81e13bc97e68c0431b26499f'
$sdkFiles = [ordered]@{
    'plugin_definitions.h'              = '09839fd1aa076df3e12a2f1d01b0c54fa00d129df6786ded4d3221f3ea7f8326'
    'ts3_functions.h'                   = '0cad4df43e0dd2cf713a2db4fcb8ca84bb2fab97e1a54502f5e62f2e49bc9643'
    'teamlog/logtypes.h'                = '7630586c5f3dfd0ab37f31b3358633d24784f9257c1d1c04defb440a673fc91c'
    'teamspeak/public_definitions.h'    = 'b41171b42e0bc525f77cbcc01d37b7e0449642fd177851564473ddd033b7f870'
}

function Fail([string]$msg) {
    Write-Host "build-teamspeak-plugin: $msg" -ForegroundColor Red
    exit 1
}

function Run([string]$exe, [string[]]$arguments) {
    & $exe @arguments
    if ($LASTEXITCODE -ne 0) { Fail "$exe failed with exit code $LASTEXITCODE" }
}

if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) { Fail 'gcc (MinGW-w64) is not on PATH' }
$target = (& gcc -dumpmachine).Trim()
if ($target -notlike 'x86_64-*mingw*') { Fail "gcc targets '$target', a 64-bit MinGW-w64 gcc is required" }

New-Item -ItemType Directory -Force -Path $build | Out-Null

# 1. SDK headers (pinned and verified).
foreach ($rel in $sdkFiles.Keys) {
    $dest = Join-Path $sdk $rel
    $want = $sdkFiles[$rel]
    $have = if (Test-Path $dest) { (Get-FileHash -Algorithm SHA256 $dest).Hash.ToLowerInvariant() } else { '' }
    if ($have -ne $want) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dest) | Out-Null
        $url = "https://raw.githubusercontent.com/TeamSpeak-Systems/ts3client-pluginsdk/$sdkCommit/include/$rel"
        Write-Host "fetching $rel"
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $dest
        $got = (Get-FileHash -Algorithm SHA256 $dest).Hash.ToLowerInvariant()
        if ($got -ne $want) {
            Remove-Item -Force $dest
            Fail "SHA-256 mismatch for $rel (expected $want, got $got)"
        }
    }
}

# 2. Core unit tests.
$testExe = Join-Path $build 'core_test.exe'
Run 'gcc' @('-Wall', '-Wextra', '-Werror', '-O1', '-D__USE_MINGW_ANSI_STDIO=1',
    (Join-Path $pluginDir 'tests\core_test.c'), (Join-Path $pluginDir 'src\core.c'), '-o', $testExe)
& $testExe
if ($LASTEXITCODE -ne 0) { Fail 'core tests failed' }

# 3. The DLL.
$dll = Join-Path $build 'cliphub_teamspeak_win64.dll'
$src = Join-Path $pluginDir 'src'
Run 'gcc' @('-O2', '-Wall', '-Wextra', '-Werror', '-shared', '-D__USE_MINGW_ANSI_STDIO=1', '-DWIN32_LEAN_AND_MEAN',
    '-I', $sdk, '-I', $src,
    (Join-Path $src 'core.c'), (Join-Path $src 'recorder.c'), (Join-Path $src 'poll.c'), (Join-Path $src 'plugin.c'),
    '-o', $dll, '-static', '-static-libgcc', '-Wl,--no-insert-timestamp', '-s', '-lwinhttp')

# 4. The .ts3_plugin package: a zip with package.ini at the root and plugins/<name>_win64.dll.
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $out) | Out-Null
if (Test-Path $out) { Remove-Item -Force $out }
Add-Type -AssemblyName System.IO.Compression
$zipStream = [System.IO.File]::Open($out, [System.IO.FileMode]::CreateNew)
try {
    $zip = New-Object System.IO.Compression.ZipArchive($zipStream, [System.IO.Compression.ZipArchiveMode]::Create)
    try {
        $entries = @(
            @{ Name = 'package.ini'; Path = (Join-Path $pluginDir 'package.ini') },
            @{ Name = 'plugins/cliphub_teamspeak_win64.dll'; Path = $dll }
        )
        foreach ($e in $entries) {
            $entry = $zip.CreateEntry($e.Name, [System.IO.Compression.CompressionLevel]::Optimal)
            $es = $entry.Open()
            try {
                $bytes = [System.IO.File]::ReadAllBytes($e.Path)
                $es.Write($bytes, 0, $bytes.Length)
            } finally { $es.Dispose() }
        }
    } finally { $zip.Dispose() }
} finally { $zipStream.Dispose() }

Write-Host "built $out"
