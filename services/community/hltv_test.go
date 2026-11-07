package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestAnalysisRoundTripAndListing(t *testing.T) {
	dir := t.TempDir()
	h := analysisHeader{Format: analysisFormat, Match: hltvSidecar{Source: hltvDataset, MatchID: 2400303, Event: "ESL Challenger League"}, Entry: "alliance-vs-maybe-m1-mirage.dem", Document: tacticalplan.NewDocument()}
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
