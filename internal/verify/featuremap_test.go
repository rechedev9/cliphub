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
		"inicio", "partidas", "subir-demo", "clips-de-stream", "demo-completa",
		"tactica", "cheaterdetect", "jugadores", "biblioteca", "ajustes",
		"shorts-9x16-wait", "full-demo-16x9-wait",
	} {
		if !seen[want] {
			t.Fatalf("catalog missing required feature %s", want)
		}
	}
}

func TestCatalogCheapFeaturesHaveProbePath(t *testing.T) {
	t.Parallel()
	for _, feature := range Features() {
		if feature.RequiresHLAECS2 || feature.RequiresWindowsStudio {
			if feature.ProbePath != "" {
				t.Fatalf("%s is an HLAE feature and must not carry probe_path %q", feature.ID, feature.ProbePath)
			}
			continue
		}
		if !strings.HasPrefix(feature.ProbePath, "/api/") {
			t.Fatalf("%s probe_path = %q, want /api/...", feature.ID, feature.ProbePath)
		}
	}
}

func TestCatalogCoversStudioNav(t *testing.T) {
	t.Parallel()
	want := []string{
		"/onboarding", "/matches", "/upload", "/streams", "/full-demo",
		"/tactical", "/cheaters", "/players", "/videos", "/settings",
	}
	have := map[string]bool{}
	for _, feature := range Features() {
		have[feature.Route] = true
	}
	for _, route := range want {
		if !have[route] {
			t.Fatalf("catalog missing nav route %s", route)
		}
	}
}

func TestCatalogUsesCurrentRailLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id    string
		label string
	}{
		{id: "partidas", label: "Demos"},
		{id: "subir-demo", label: "Shorts"},
		{id: "clips-de-stream", label: "Clips de stream"},
		{id: "demo-completa", label: "Vídeos largos"},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			feature, ok := FeatureByID(tc.id)
			if !ok {
				t.Fatalf("FeatureByID(%q) not found", tc.id)
			}
			if feature.NavLabel != tc.label {
				t.Fatalf("FeatureByID(%q).NavLabel = %q, want %q", tc.id, feature.NavLabel, tc.label)
			}
		})
	}
}

func TestRemovedStudioFeaturesAreNotCatalogued(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"editor", "feed"} {
		t.Run(id, func(t *testing.T) {
			if feature, ok := FeatureByID(id); ok {
				t.Fatalf("FeatureByID(%q) = %#v, want removed feature", id, feature)
			}
		})
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
	host := ClassifyHost(HostFacts{GOOS: "linux", GOARCH: "amd64"})
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
	_, err := ProveFeature(".", ClassifyHost(HostFacts{GOOS: "linux", GOARCH: "amd64"}), "not-a-feature")
	if err == nil {
		t.Fatal("expected unknown feature error")
	}
}

func TestProveInicioCheapProof(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	report, err := ProveFeature(root, ClassifyHost(HostFacts{GOOS: "linux", GOARCH: "amd64"}), "inicio")
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
	if report.Drive == nil || report.Drive.Route != "/onboarding" || report.Drive.NavLabel != "Inicio" {
		t.Fatalf("drive = %#v, want Inicio /onboarding", report.Drive)
	}
}
