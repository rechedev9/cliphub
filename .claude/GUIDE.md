# Claude Code Guide

Guidance for using Claude Code safely on TickCut.

Claude Code automatically loads `CLAUDE.md` from the repo root.
That file holds project boundaries, Go and TypeScript style, safety rules, and verification expectations.
All style and operational rules live directly in `CLAUDE.md`.

Studio ships no assistant surface of its own; it is a GUI over the same pipeline.
Claude Code is a repository-development tool and is not part of the shipped product.
The former external MCP registration is no longer part of the product.

## Use

Run Claude Code from the repository root in Git Bash, not through the broken bare `bash` WSL shim:

```bash
claude
```

Use the unified CLI for TickCut operations and inspect its executable contracts before composing commands:

```powershell
.\bin\zv.exe capabilities --format json
.\bin\zv.exe flows show demo --format json
.\bin\zv.exe workflows show short --format json
.\bin\zv.exe skills list --format json
```

For repository verification, run the relevant project gates from Git Bash:

```bash
scripts/go-gate.sh --no-format
scripts/go-gate.sh --race
scripts/go-gate.sh --security
scripts/check-codex-harness.sh
```

## Safety defaults

- `.claude/settings.json` allows normal repository work and Go checks.
- It asks before dependency, git-history, Docker, FFmpeg, PowerShell, build, and cleanup operations.
- It denies secret reads and destructive Git or filesystem commands.
- Do not use `--dangerously-skip-permissions` for this repo unless you have an external sandbox.
