---
name: update-golden
description: Rewrite and review the internal/pipeline golden files. Use when go test ./internal/pipeline fails with a golden mismatch after a change that was meant to move the Short pipeline output, or when a new behavior needs a case in the pipeline golden test.
---

# Update pipeline goldens

The rule is [Render and the editor](../../rules/render-editor.md), section Pipeline golden test.

## Steps

1. Decide whether the mismatch is intended. If the change was not meant to move the output, fix the code and stop; do not rewrite the goldens.
2. For new behavior, extend `internal/pipeline/short_pipeline_test.go` and its `testdata/`. Do not add a second pipeline suite, a script or an evidence folder.
3. Rewrite the goldens:

```bash
go test ./internal/pipeline -update
```

4. Read `git diff internal/pipeline/testdata` the way you review code. Accept only the lines the change was meant to move. Never accept the diff blindly: a golden that moved for a reason you cannot explain is a bug to fix.

## Checks

```bash
go test ./internal/pipeline -count=1
go vet ./...
```

The update is finished when the test is green without `-update` and every changed golden line is explained by the change.
