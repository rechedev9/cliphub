package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestAnalysisRoundTripAndListing(t *testing.T) {
	dir := t.TempDir()
	h := community.ProAnalysisHeader{Format: community.ProAnalysisFormat, Match: community.ProSidecar{Source: hltvDataset, MatchID: 2400303, Event: "ESL Challenger League"}, Entry: "alliance-vs-maybe-m1-mirage.dem", Document: tacticalplan.NewDocument()}
	h.Document.Demo.Map = "de_mirage"
	blob := bytes.Repeat([]byte{1, 2, 3}, 1000)
	name := filepath.Join(dir, "2400303__alliance-vs-maybe-m1-mirage"+analysisSuffix)
	if err := writeAnalysis(name, h, blob); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, gotBlob, err := readAnalysis(name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Match.MatchID != 2400303 || got.Document.Demo.Map != "de_mirage" || !bytes.Equal(gotBlob, blob) {
		t.Fatalf("round trip lost data: %+v", got.Match)
	}
	files, err := listAnalyses(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "2400303/alliance-vs-maybe-m1-mirage.dem" {
		t.Fatalf("listing = %+v", files)
	}
	if analysisFileName(files[0].Path) != filepath.Base(name) {
		t.Fatalf("file name does not round trip: %s", analysisFileName(files[0].Path))
	}
	if mapKey(files[0].Path) != mapKey("2400303/alliance-vs-maybe-m1-mirage-p2.dem") {
		t.Fatal("restart parts of one map must share a key")
	}
}

func TestWriteSidecarsGivesOneSidecarPerMatch(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	for _, entry := range []string{"a-vs-b-m1-mirage", "a-vs-b-m2-nuke"} {
		h := community.ProAnalysisHeader{Format: community.ProAnalysisFormat, Match: community.ProSidecar{Source: hltvDataset, MatchID: 2400303, DemoPath: "/download/demo/99", Event: "IEM"}, Entry: entry + ".dem", Document: tacticalplan.NewDocument()}
		if err := writeAnalysis(filepath.Join(dir, "2400303__"+entry+analysisSuffix), h, []byte{1}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := writeSidecars(dir, out)
	if err != nil || n != 1 {
		t.Fatalf("wrote %d sidecars: %v", n, err)
	}
	body, err := os.ReadFile(filepath.Join(out, "2400303.json"))
	if err != nil || !bytes.Contains(body, []byte(`"demo_path":"/download/demo/99"`)) {
		t.Fatalf("sidecar = %s %v", body, err)
	}
}
