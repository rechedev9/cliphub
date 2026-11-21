---
name: post-work-review
description: Review and fix the staged ClipHub change before each commit, run the checks for what it touches, then record the reviewed tree so .githooks/pre-commit lets the commit through. Use after any code change, before every commit an agent makes in this repo.
---

# Post-work review

Run it on the change you are about to commit, after the work is done. Unlike [Diff review](../diff-review/SKILL.md), this skill fixes what it finds. `/code-review high --fix` is separate and runs once before a PR.

## 1. Stage the change

Stage exactly what the commit will hold. Leave other people's edits and `scratch.txt` unstaged. Review with `git diff --cached`.

## 2. Check every item

- Every item of [Diff review](../diff-review/SKILL.md) on the staged diff: MCP parity, the 1000-line cap, dead code, store contracts, no e2e or evidence files, no secrets, rules moving with the code.
- TypeScript: no `any`, `as` (except `as const`), `!`, `@ts-ignore`, `@ts-expect-error` or lint disables outside test mocks. No re-exports, one block of static imports at the top, and a typed options object for more than two parameters.
- Comments: none. New code has no comments and a frozen file never gains one (`TestNoComments`, oxlint `cliphub/no-comments`). After you delete comments, lower the baselines as the failure message says.
- UI copy is Spanish with accents and no em or en dashes. A user-visible change was checked by hand in a throwaway profile ([Verify in Studio](../verify-in-studio/SKILL.md)).
- Each new or changed test catches a regression you can name. A test that only pins authored text or counts is rewritten to the invariant or deleted.
- A fix for a production failure adds its incident to `docs/incidents.md` when the rule files point there.

## 3. Run the checks

Run the checks of each area the diff touches ([Build and check](../../../AGENTS.md#build-and-check)):

- Go: `go vet ./...`, `go test` of the touched packages, and `go test ./cmd/zv -run 'TestGoFileLength|TestNoDeadCode' -count=1` when a Go function is added or removed.
- `web`, `desktop`, `landing`, `mcp-apps`: that package's `typecheck`, `lint` and `test:unit`.

Fix what the change introduced and stage the fix. Report a failure outside the change instead of fixing it here.

## 4. Record the review

When nothing is left to fix and the staged tree is final:

```bash
git write-tree > "$(git rev-parse --git-path post-work-review)"
```

`.githooks/pre-commit` compares this with the tree being committed and refuses an agent's commit (`CLAUDECODE` set) that does not match. Staging anything afterwards means running this skill again. Say in the reply what the review found and fixed.
