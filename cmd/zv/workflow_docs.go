package main

// workflowDocs retains its CLI JSON name but only validates executable sources.
// Product behavior lives in the command catalog and these build/run scripts.
func workflowDocs() []workflowDoc {
	return []workflowDoc{
		{
			Path: "scripts/smoke-real.ps1",
			Required: []string{
				`bin\zv serve`,
			},
		},
		{
			Path: "scripts/smoke.sh",
			Required: []string{
				"ZV_BASE_URL",
				"/api/jobs",
				"/api/jobs/$ID",
				"/api/jobs/$ID/plan",
			},
		},
		{
			Path: "Makefile",
			Required: []string{
				"go build -o bin/zv ./cmd/zv",
				"go run ./cmd/zv check",
				"go run ./cmd/zv workflows check",
			},
		},
		{
			Path: "scripts/build.ps1",
			Required: []string{
				`"zv"`,
				"& go build -o $out $pkg",
			},
		},
		{
			Path: "scripts/go-gate.sh",
			Required: []string{
				"== zv check ==",
				"go run ./cmd/zv check",
			},
		},
		{
			Path: "scripts/fix-loop.ps1",
			Required: []string{
				`Invoke-Step "zv check"`,
				"go run ./cmd/zv check",
			},
		},
	}
}
