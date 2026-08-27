[CmdletBinding()]
param(
    [ValidateSet("Doctor", "Preview", "Run", "Check")]
    [string]$Action = "Preview",

    [ValidateSet("plan", "tdd", "bugfix", "ready", "spike")]
    [string]$Playbook = "plan",

    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Task
)

$ErrorActionPreference = "Stop"
$root = (& git rev-parse --show-toplevel 2>$null)
if (-not $root) {
    throw "Codex harness requires a Git repository"
}
$root = $root.Trim()

$prompts = @{
    plan   = ".codex/prompts/go-plan.md"
    tdd    = ".codex/prompts/go-tdd.md"
    bugfix = ".codex/prompts/go-bugfix.md"
    ready  = ".codex/prompts/go-pr-ready.md"
    spike  = ".codex/prompts/go-spike.md"
}

function Get-GitBash {
    $candidates = @(
        "C:\Program Files\Git\bin\bash.exe",
        "C:\Program Files\Git\usr\bin\bash.exe"
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate) {
            return $candidate
        }
    }
    throw "Git Bash was not found under C:\Program Files\Git"
}

function Test-AgentSymlink {
    $agents = Get-Item -LiteralPath (Join-Path $root "AGENTS.md") -Force
    if ($agents.LinkType -ne "SymbolicLink" -or $agents.Target -notcontains "CLAUDE.md") {
        throw "AGENTS.md must be a symbolic link to CLAUDE.md"
    }
}

switch ($Action) {
    "Doctor" {
        Test-AgentSymlink
        $codex = Get-Command codex.cmd -ErrorAction SilentlyContinue
        if (-not $codex) {
            $codex = Get-Command codex.exe -ErrorAction Stop
        }
        $bash = Get-GitBash
        Write-Output "repository: $root"
        Write-Output "codex: $($codex.Path)"
        & $codex.Path --version
        Write-Output "git-bash: $bash"
        Write-Output "AGENTS.md: symbolic link to CLAUDE.md"
        exit 0
    }
    "Check" {
        Test-AgentSymlink
        Push-Location $root
        try {
            & go run ./cmd/zv check
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        } finally {
            Pop-Location
        }
        exit 0
    }
}

$bash = Get-GitBash
$runner = (Join-Path $root "scripts/codex-run.sh").Replace("\", "/")
$arguments = @($runner)
if ($Action -eq "Run") {
    $arguments += "--execute"
}
$arguments += $prompts[$Playbook]
$arguments += $Task

Push-Location $root
try {
    & $bash @arguments
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
