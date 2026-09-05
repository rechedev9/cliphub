package verify

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestFeatureMapUsesCompiledCatalogWithoutDocuments(t *testing.T) {
	report := InspectFeatureMap(t.TempDir())
	if !report.OK {
		t.Fatalf("feature map issues: %s", strings.Join(report.Issues, "; "))
	}
	if report.Source != FeatureCatalogSource {
		t.Fatalf("source = %q", report.Source)
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
		if !feature.CatalogValid || !feature.CheapOK {
			t.Fatalf("feature %s catalog_valid=%t cheap_ok=%t issues=%v", feature.ID, feature.CatalogValid, feature.CheapOK, feature.Issues)
		}
		if feature.RequiresHLAECS2 && feature.UserPath == "pass" {
			t.Fatalf("feature %s leaked a Pass user_path", feature.ID)
		}
	}
	for _, want := range []string{
		"inicio", "partidas", "subir-demo", "demo-completa", "tactica",
		"cheaterdetect", "jugadores", "clips-de-stream", "editor",
		"biblioteca", "feed", "ajustes",
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

func TestFeatureCatalogValidationRejectsIncompleteMetadata(t *testing.T) {
	valid := Features()[0]
	cases := []struct {
		name     string
		features []Feature
	}{
		{name: "empty"},
		{name: "duplicate", features: []Feature{valid, valid}},
		{name: "missing identity", features: []Feature{{Route: "/onboarding", ProbePath: "/api/steam/account"}}},
		{name: "external route", features: []Feature{{ID: "external", Title: "External", Route: "//example.com", CheapProof: "probe", ProbePath: "/api/test"}}},
		{name: "missing probe", features: []Feature{{ID: "missing-probe", Title: "Missing probe", Route: "/onboarding", CheapProof: "probe"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := inspectFeatures(tc.features)
			if report.OK || len(report.Issues) == 0 {
				t.Fatalf("invalid catalog accepted: %#v", report)
			}
		})
	}
}

func TestCatalogCoversStudioNav(t *testing.T) {
	t.Parallel()
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	nav, err := os.ReadFile(filepath.Join(root, "web", "lib", "nav.ts"))
	if err != nil {
		t.Fatal(err)
	}
	// Read the actual rail so this check cannot bless another stale copy.
	sections := regexp.MustCompile(`\{ number: '[0-9]+', label: '([^']+)', href: '([^']+)' \}`).FindAllStringSubmatch(string(nav), -1)
	if len(sections) == 0 {
		t.Fatal("no Studio nav sections found")
	}
	have := map[string]bool{}
	for _, feature := range Features() {
		route, err := url.Parse(feature.Route)
		if err != nil {
			t.Fatalf("%s route: %v", feature.ID, err)
		}
		page := filepath.Join(root, "web", "app", "(app)", filepath.FromSlash(strings.TrimPrefix(route.Path, "/")), "page.tsx")
		body, err := os.ReadFile(page)
		if err != nil {
			t.Fatalf("%s route %s has no Studio page: %v", feature.ID, feature.Route, err)
		}
		if strings.Contains(string(body), "redirect(") {
			t.Errorf("%s points to retired route %s", feature.ID, feature.Route)
		}
		if feature.NavLabel == "" {
			continue
		}
		matched := false
		for _, section := range sections {
			if feature.NavLabel == section[1] && (route.Path == section[2] || strings.HasPrefix(route.Path, section[2]+"/")) {
				matched = true
				have[section[2]] = true
			}
		}
		if !matched {
			t.Errorf("%s route %s has no matching Studio nav label %q", feature.ID, feature.Route, feature.NavLabel)
		}
	}
	for _, section := range sections {
		if !have[section[2]] {
			t.Errorf("catalog missing nav route %s (%s)", section[2], section[1])
		}
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
	if report.Drive == nil || report.Drive.Route != "/clips" || report.Drive.NavLabel != "Clips y vídeos" {
		t.Fatalf("drive = %#v, want Clips y vídeos /clips", report.Drive)
	}
}
