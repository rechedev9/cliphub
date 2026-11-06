package tacticalplan_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

var updateProReference = flag.Bool("update-pro-reference", false, "rewrite pro_reference_default.json from the demos in CLIPHUB_PRO_DEMOS")

// The demos are local files that cannot be versioned, so the description names
// them; change it whenever the demo list changes.
const (
	proReferenceID          = "pro-2026-isog-lavked-tnc"
	proReferenceDescription = "4 mapas profesionales de 2026: Iberian Soul contra OG (Anubis, Nuke e Inferno) y Lavked contra TNC (Nuke)."
)

// TestGenerateProReference measures the bundled pro reference. Run it with
//
//	CLIPHUB_PRO_DEMOS="a.dem;b.dem" go test ./internal/tacticalplan -run TestGenerateProReference -update-pro-reference -timeout 30m
func TestGenerateProReference(t *testing.T) {
	if !*updateProReference {
		t.Skip("run with -update-pro-reference and CLIPHUB_PRO_DEMOS to regenerate the bundled pro reference")
	}
	paths := filepath.SplitList(os.Getenv("CLIPHUB_PRO_DEMOS"))
	if len(paths) == 0 {
		t.Fatal("CLIPHUB_PRO_DEMOS lists no demo")
	}
	build := tacticalplan.ProReferenceBuild{ID: proReferenceID, Description: proReferenceDescription}
	for _, path := range paths {
		res, err := tactical.ScanFile(context.Background(), path, tactical.Options{})
		if err != nil {
			t.Fatalf("scan %s: %v", path, err)
		}
		summary := tacticalplan.SummarizeProDemo(res.Document)
		build.Demos = append(build.Demos, summary)
		t.Logf("%s: %s, %s, %d rounds", filepath.Base(path), summary.Match.Map, strings.Join(summary.Match.Teams, " vs "), summary.Match.Rounds)
	}
	ref, err := tacticalplan.BuildProReference(build)
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tacticalplan.DecodeProReference(out); err != nil {
		t.Fatalf("generated reference does not load: %v", err)
	}
	if err := os.WriteFile("pro_reference_default.json", append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
