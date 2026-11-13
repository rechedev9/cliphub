---
name: update-golden
description: Rewrite and review the internal/pipeline golden files when the deterministic Short pipeline output changes. Use when go test ./internal/pipeline fails on a golden diff that the change intended.
---

# Update pipeline goldens

The rule is [Render and the editor](../../rules/render-editor.md). This test is the one allowed end-to-end check: plain `go test`, goldens in `internal/pipeline/testdata/`, no CS2, network, real clock or video encode, and no scripts beside it. It does not replace [Verify in Studio](../verify-in-studio/SKILL.md).

## Steps

1. Extend `internal/pipeline/short_pipeline_test.go` (and its `testdata/`) for the new behavior. Do not add a second pipeline suite, a script, or an evidence folder.
2. Rewrite the goldens:

```bash
go test ./internal/pipeline -update
```

3. Review the golden diff the way you review code. Accept only the lines the change was meant to move. Revert a golden that moved for a reason you cannot explain.
4. Re-run without `-update` so the committed files match.

## Checks

```bash
go test ./internal/pipeline -count=1
go vet ./...
```

The update is finished when `go test ./internal/pipeline -count=1` is green and the golden diff has been reviewed.
