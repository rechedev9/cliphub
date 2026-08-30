<!--
Optional linked context:
Add a visible `Closes #<issue-number>` or `Related: #<issue-number>` line
below this comment.

Required PR title:
type: user-facing description
Use a parenthesized scope only when it adds clarity:
fix(parser): 9:16 Shorts fail to load when the demo has no kills

Types: feat, fix, improve, refactor, docs, chore.
For fixes, describe the user-visible symptom and trigger:
fix: Full Demo recap aborts capture when a buy-time nade is thrown
Avoid implementation details such as:
fix: add null check to recap planner
-->

<details>
<summary>Additional instructions</summary>

**MUST:** Keep **Allow edits from maintainers** enabled for this PR so maintainers
can help update the branch when needed.

</details>

## What Problem This Solves

<!--
Describe the concrete user, product, or operational problem.
For fixes, begin with:
"Fixes an issue where users <do X> would <experience Y> when <condition>."
or:
"Resolves a problem where..."

Name the affected ClipHub surface: Demo parser → 9:16 Shorts, Full Demo → 16:9 recap, HLAE/capture, Studio installer/updater, or docs/CI.
Do not describe the code-level cause here.
-->

## Why This Change Was Made

<!--
In one or two sentences, explain the complete shipped solution, key design
decisions, and relevant boundaries or non-goals. Include implementation detail
only when it helps reviewers understand user-visible behavior or risk.
Avoid file-by-file narration.
-->

## User Impact

<!--
State what users, operators, or developers can now do or expect. Lead with the
concrete benefit and use user-facing language. If there is no user-visible
impact, say so plainly.
-->

## Evidence

<!--
Show the most useful proof that this change works. Screenshots, screencasts,
terminal output, focused tests, CI results, live observations, redacted logs,
and artifact links are all useful. Include before/after evidence for visual
changes when it clarifies the result.

For parser/capture/render changes: real ClipHub Studio on Windows + HLAE/CS2.
Unit tests, mocks, lint, and CI are supplemental only. Name which structural
flow you exercised (9:16 Shorts and/or Full Demo 16:9). If E2E cannot run, say
so plainly. Do not claim the path works from reading code.

Reviewers will inspect the code, tests, and CI. Use this section to make the
validation easy to understand, not to restate the diff.
-->
