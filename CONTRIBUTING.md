# Contributing to ClipHub

ClipHub Studio is a Windows desktop app that turns CS2 demos and streams into edited videos.
Outside contributions arrive as **GitHub issues**, and the ClipHub team writes the fix. Issues are
usually filed by an AI agent (Claude Code, Codex or similar) on behalf of a Studio user. This file
is written for that agent.

Your job is to file an issue that a maintainer can act on without asking a single question back.
The bar is the same evidence our pull requests carry: what happened, who it hurts, and proof.

## Before you write

1. Search for an existing issue: `gh issue list --repo rechedev9/cliphub --state all --search "<words from the error>"`.
   If one matches, add your evidence as a comment on it instead of opening a new one.
2. File one issue per problem. Two unrelated failures are two issues.
3. Collect the evidence below. If you drive Studio through its MCP server (`cliphub-studio`), call
   `studio_status`: it returns `studio_version`, `support_code` and `log_path`. Otherwise ask the
   user for them; the version and support code are in Studio, Ajustes.

## Evidence

- **Studio version**, such as `5.10.0`.
- **Support code**, such as `CH-1A2B-3C4D-5E6F-7A8B-9C0D`. It lets the team read this
  installation's diagnostics. It contains no personal data.
- **Exact error**: the text Studio or the MCP tool returned, copied, not paraphrased.
- **studio.log excerpt**: the lines around the failure (its time, plus or minus a minute), not the
  whole file. The default path is `%APPDATA%\cliphub-studio\studio.log`.
- **Steps**: what the user or you did, in order, from opening Studio. For an MCP flow, list the
  tool calls with their arguments and the result of the one that failed.
- **Input**: the kind of demo or stream (map, Valve, FACEIT or HLTV demo, Twitch or YouTube link)
  and, if the user agrees to share it, a download link. Do not attach demos you were not given.
- **Frequency**: always, sometimes (how often) or once.

## Redact

The repository is public. Before you submit:
- Replace the Windows user name in paths with `<user>`.
- Remove tokens, cookies, API keys and the contents of `agent-session.json`.
- Keep player names and SteamIDs only when the bug depends on them.

## Write the issue

Use the template: `gh issue create --repo rechedev9/cliphub --template bug-report.md`, or pass
`--title` and `--body-file` with the same sections. Write in English or Spanish.

The title says what fails in user terms: "Full Demo render stops at 0 % on Anubis", not "Bug in
render". A feature request uses the same sections: the problem it solves goes under What
Happened, and Evidence shows how the user works around it today.

Keep what you observed apart from what you suspect. Facts go under Evidence. A guess about the
cause goes under Hypothesis, marked as a guess, with the file or log line that led you there. Do
not present an unverified root cause or a proposed patch as the answer.

## Done when

- [ ] No existing issue covers it.
- [ ] Every section of the template is filled, or says why it cannot be.
- [ ] The error text and log excerpt are copied, not summarized.
- [ ] Paths and secrets are redacted.
- [ ] The body says which agent and model filed it, and whether a person reproduced the problem.
