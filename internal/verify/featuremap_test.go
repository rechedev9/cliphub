package verify

import (
	"strings"
	"testing"
)

func TestFeatureMapExistsAndMatchesCatalog(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	report := InspectFeatureMap(root)
	if !report.OK {
		t.Fatalf("feature map issues: %s", strings.Join(report.Issues, "; "))
	}
	if !report.IndexPresent {
		t.Fatal("missing references/features/INDEX.md")
	}
	if got, want := len(report.Features), len(Features()); got != want {
		t.Fatalf("features = %d, want %d", got, want)
	}
	seen := map[string]bool{}
	for _, feature := range report.Features {
		if seen[feature.ID] {
			t.Fatalf("duplicate feature %s", feature.ID)
		}
		seen[feature.ID] = true
		if !feature.MapPresent || !feature.CheapOK {
			t.Fatalf("feature %s map_present=%t cheap_ok=%t issues=%v", feature.ID, feature.MapPresent, feature.CheapOK, feature.Issues)
		}
		if got, want := len(feature.Headings), len(RequiredFeatureHeadings); got != want {
			t.Fatalf("feature %s headings = %d, want %d", feature.ID, got, want)
		}
		if feature.RequiresHLAECS2 && feature.UserPath == "pass" {
			t.Fatalf("feature %s leaked a Pass user_path", feature.ID)
		}
	}
	for _, want := range []string{
		"inicio", "partidas", "subir-demo", "demo-completa", "cheaterdetect",
		"tactica", "jugadores", "clips-de-stream", "biblioteca",
		"shorts-9x16-wait", "full-demo-16x9-wait",
	} {
		if !seen[want] {
			t.Fatalf("catalog missing required feature %s", want)
		}
	}
}

func TestInspectSkillContract(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	skill := InspectSkill(root)
	if !skill.OK {
		t.Fatalf("skill issues: %s", strings.Join(skill.Issues, "; "))
	}
	if skill.Name != "verify-cliphub" {
		t.Fatalf("skill name = %q", skill.Name)
	}
	if skill.Description == "" {
		t.Fatal("empty skill description")
	}
}

func TestProveFeatureFailsClosedForCapture(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	host := ClassifyHost("linux", "amd64", false, false)
	tests := []string{"demo-completa", "shorts-9x16-wait", "full-demo-16x9-wait"}
	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			report, err := ProveFeature(root, host, id)
			if err != nil {
				t.Fatal(err)
			}
			if report.OK || !report.Closed || report.Gap == nil {
				t.Fatalf("prove = %#v, want fail closed with gap", report)
			}
			if report.Gap.ID != ClosedCaptureGapID || !strings.Contains(report.Gap.Message, "Cloud Linux") {
				t.Fatalf("gap = %#v, want named HLAE/CS2 gap", report.Gap)
			}
		})
	}
}

func TestProveUnknownFeature(t *testing.T) {
	_, err := ProveFeature(".", ClassifyHost("linux", "amd64", false, false), "not-a-feature")
	if err == nil {
		t.Fatal("expected unknown feature error")
	}
}

func TestProveInicioCheapProof(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	report, err := ProveFeature(root, ClassifyHost("linux", "amd64", false, false), "inicio")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("inicio cheap proof failed: %#v", report)
	}
	if report.Closed {
		t.Fatal("inicio must not close the HLAE gap")
	}
	if !strings.Contains(report.Detail, "unproven") {
		t.Fatalf("detail = %q, want an honest unproven user-path note", report.Detail)
	}
}
