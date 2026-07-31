package tactical

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/rechedev9/tickcut/internal/tacticalplan"
)

func TestOptionsSampleRate(t *testing.T) {
	tests := []struct {
		name    string
		hz      float64
		want    float64
		wantErr bool
	}{
		{"zero selects the default", 0, DefaultSampleHZ, false},
		{"explicit rate", 16, 16, false},
		{"at the ceiling", MaxSampleHZ, MaxSampleHZ, false},
		{"above the ceiling", MaxSampleHZ + 1, 0, true},
		{"negative", -1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Options{SampleHZ: tt.hz}.sampleHZ()
			if tt.wantErr {
				if err == nil {
					t.Fatal("want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("sampleHZ: %v", err)
			}
			if got != tt.want {
				t.Fatalf("sampleHZ = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndReasonSlugsAreStable(t *testing.T) {
	// These strings are a filter vocabulary the UI and the CLI both match on,
	// so they must never drift with a library upgrade.
	if got := endReasonSlug(0); got != "unknown" {
		t.Fatalf("an unmapped reason = %q, want %q", got, "unknown")
	}
}

func TestTeamKeyFollowsTheStartingSide(t *testing.T) {
	// In the second half a player's current side is the opposite of the side
	// their team started on, and the key must not flip with it.
	if got := teamKeyForStartSide(tacticalplan.SideCT, 1); got != "ct-start" {
		t.Fatalf("first-half CT = %q, want ct-start", got)
	}
	if got := teamKeyForStartSide(tacticalplan.SideCT, 2); got != "t-start" {
		t.Fatalf("second-half CT = %q, want t-start", got)
	}
	if got := teamKeyForStartSide(tacticalplan.SideT, 2); got != "ct-start" {
		t.Fatalf("second-half T = %q, want ct-start", got)
	}
}

// TestScanRealDemo exercises the whole pipeline against a real recording. It
// skips without TEST_DEMO_PATH, the way the other demo-backed tests in this
// repository do, because real demos are not committed.
func TestScanRealDemo(t *testing.T) {
	path := os.Getenv("TEST_DEMO_PATH")
	if path == "" {
		t.Skip("set TEST_DEMO_PATH to a .dem file to run the end-to-end scan")
	}

	result, err := ScanFile(context.Background(), path, Options{})
	if err != nil {
		t.Fatalf("ScanFile: %v", err)
	}
	doc := result.Document

	if doc.SchemaVersion != tacticalplan.SchemaVersion {
		t.Fatalf("schema version = %q, want %q", doc.SchemaVersion, tacticalplan.SchemaVersion)
	}
	if doc.Demo.Map == "" || doc.Demo.Tickrate <= 0 {
		t.Fatalf("demo metadata is incomplete: %+v", doc.Demo)
	}
	if len(doc.Rounds) == 0 {
		t.Fatal("a real demo must produce rounds")
	}
	if len(doc.Players) < 2 {
		t.Fatalf("player table has %d entries", len(doc.Players))
	}

	// Slots must be dense and ordered, because both the blob and every event
	// reference them by index.
	for i, p := range doc.Players {
		if int(p.Slot) != i {
			t.Fatalf("player %d has slot %d; slots must be dense", i, p.Slot)
		}
		if p.SteamID64 == "" || p.StartSide == "" {
			t.Fatalf("player %d is missing identity: %+v", i, p)
		}
	}

	classified := 0
	for _, r := range doc.Rounds {
		if r.TickEnd <= r.TickStart {
			t.Fatalf("round %d has no duration: %d..%d", r.Number, r.TickStart, r.TickEnd)
		}
		if r.Economy.CTBuy == "" || r.Economy.TBuy == "" {
			t.Fatalf("round %d has no economy classification", r.Number)
		}
		if len(r.Class.Reasons) == 0 {
			t.Fatalf("round %d was classified without recording a reason", r.Number)
		}
		if r.Class.TSide != tacticalplan.TUnknown || r.Class.CTSide != tacticalplan.CTUnknown {
			classified++
		}
		if r.Bomb != nil && r.Bomb.PlantTick > 0 && r.Class.Site == tacticalplan.SiteNone {
			t.Fatalf("round %d has a plant but no site", r.Number)
		}
	}
	if classified == 0 {
		t.Fatal("no round in the demo was classified beyond unknown")
	}

	// The position blob must decode, and every round must be seekable inside it.
	if result.Positions.Descriptor.FrameCount == 0 {
		t.Fatal("no positions were sampled")
	}
	desc, frames, err := tacticalplan.DecodePositions(result.Positions.Data)
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	if len(frames) != result.Positions.Descriptor.FrameCount {
		t.Fatalf("decoded %d frames, descriptor says %d", len(frames), result.Positions.Descriptor.FrameCount)
	}
	for _, r := range doc.Rounds {
		offset, ok := doc.Positions.Offset(r.Number)
		if !ok {
			t.Fatalf("round %d has no position offset", r.Number)
		}
		if offset.FrameCount == 0 {
			continue
		}
		roundFrames, err := tacticalplan.DecodeFrames(result.Positions.Data, offset.ByteOffset, offset.FrameCount, desc)
		if err != nil {
			t.Fatalf("seeking round %d: %v", r.Number, err)
		}
		if roundFrames[0].Tick != offset.FirstTick {
			t.Fatalf("round %d starts at tick %d, offset says %d", r.Number, roundFrames[0].Tick, offset.FirstTick)
		}
	}

	// The derived radar must be drawable: cells to fill and callouts to label.
	if len(doc.Geometry.Levels) == 0 || len(doc.Geometry.Levels[0].Cells) == 0 {
		t.Fatal("the occupancy geometry is empty; there would be nothing to draw")
	}
	if !doc.Geometry.Calibration.Valid() {
		t.Fatalf("geometry calibration is unusable: %+v", doc.Geometry.Calibration)
	}

	// The document is the durable artifact, so it must round-trip through JSON.
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	var decoded tacticalplan.Document
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	if len(decoded.Rounds) != len(doc.Rounds) {
		t.Fatalf("round trip lost rounds: %d -> %d", len(doc.Rounds), len(decoded.Rounds))
	}
}

// TestScanRealDemoIsDeterministic runs the same demo twice: a tactical document
// that changes between runs cannot be the basis of a scouting report.
func TestScanRealDemoIsDeterministic(t *testing.T) {
	path := os.Getenv("TEST_DEMO_PATH")
	if path == "" {
		t.Skip("set TEST_DEMO_PATH to a .dem file to run the determinism check")
	}

	first, err := ScanFile(context.Background(), path, Options{})
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	second, err := ScanFile(context.Background(), path, Options{})
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}

	// GeneratedAt is a timestamp by design; everything else must match.
	first.Document.GeneratedAt = second.Document.GeneratedAt
	a, err := json.Marshal(first.Document)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	b, err := json.Marshal(second.Document)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("two scans of the same demo produced different documents")
	}
	if first.Positions.Descriptor.SHA256 != second.Positions.Descriptor.SHA256 {
		t.Fatal("two scans of the same demo produced different position blobs")
	}
}
