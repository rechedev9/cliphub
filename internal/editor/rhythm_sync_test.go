package editor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rhythm"
)

func loadRhythmSync(path string) (map[string]rhythm.SegmentSync, error) {
	analysis, err := readRhythmAnalysis(path)
	if err != nil || analysis == nil {
		return nil, err
	}
	return indexRhythmSync(*analysis)
}

func TestLoadRhythmSyncIndexesSegments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rhythm.json")
	if err := os.WriteFile(path, []byte(`{
		"schema_version":"1.0",
		"segment_sync":[
			{"segment_id":"seg-001","timeline_start_seconds":0.5,"target_kill_time_seconds":1.5},
			{"segment_id":"seg-002","timeline_start_seconds":4.0,"target_kill_time_seconds":5.0}
		]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	sync, err := loadRhythmSync(path)
	if err != nil {
		t.Fatalf("loadRhythmSync returned error: %v", err)
	}
	if got := sync["seg-002"].TimelineStartSeconds; got != 4.0 {
		t.Fatalf("seg-002 timeline start = %.3f, want 4.000", got)
	}
}

func TestLoadRhythmSyncRejectsEmptySegmentSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rhythm.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"1.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadRhythmSync(path)
	if err == nil {
		t.Fatal("loadRhythmSync returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "segment_sync") {
		t.Fatalf("error = %v, want segment_sync context", err)
	}
}

func TestLoadRhythmSyncRejectsMissingSegmentID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rhythm.json")
	if err := os.WriteFile(path, []byte(`{
		"schema_version":"1.0",
		"segment_sync":[{"timeline_start_seconds":0.5}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadRhythmSync(path)
	if err == nil {
		t.Fatal("loadRhythmSync returned nil error, want error")
	}
	if !strings.Contains(err.Error(), "without segment_id") {
		t.Fatalf("error = %v, want missing segment_id context", err)
	}
}

func TestLoadRhythmSyncNormalizesTailTrimLikeTheProducer(t *testing.T) {
	dir := t.TempDir()
	musicPath := filepath.Join(dir, "music.mp3")
	music := []byte("music fixture")
	if err := os.WriteFile(musicPath, music, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		TickStart: 0,
		TickEnd:   640,
		Kills:     []killplan.Kill{{Tick: 320}},
	}}
	const rawTailTrim = 1.2345
	normalized := rhythm.NormalizeTailTrimSeconds(rawTailTrim)
	analysis := rhythm.Analysis{
		SchemaVersion:     rhythm.SchemaVersion,
		SourceSHA256:      fmt.Sprintf("%x", sha256.Sum256(music)),
		KillOffsetSeconds: 0.1,
		TailTrimSeconds:   normalized,
		BeatTimesSeconds:  []float64{0.5, 1, 1.5},
		SegmentSync:       rhythm.BuildSegmentSyncWithTrim(plan, []float64{0.5, 1, 1.5}, 0.1, normalized),
	}
	body, err := json.Marshal(analysis)
	if err != nil {
		t.Fatal(err)
	}
	rhythmPath := filepath.Join(dir, "rhythm.json")
	if err := os.WriteFile(rhythmPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	sync, err := loadRhythmSyncForRender(rhythmPath, musicPath, plan, rawTailTrim)
	if err != nil {
		t.Fatalf("loadRhythmSyncForRender returned error: %v", err)
	}
	if _, ok := sync["seg-001"]; !ok {
		t.Fatal("normalized rhythm sync omitted seg-001")
	}
}

func TestLoadRhythmSyncForRenderAcceptsOnlyAnOrderedAnalysisPrefix(t *testing.T) {
	dir := t.TempDir()
	musicPath := filepath.Join(dir, "music.mp3")
	music := []byte("music fixture")
	if err := os.WriteFile(musicPath, music, 0o600); err != nil {
		t.Fatal(err)
	}

	fullPlan := killplan.NewPlan()
	fullPlan.Demo.Tickrate = 64
	fullPlan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 320}}},
		{ID: "seg-002", TickStart: 640, TickEnd: 1280, Kills: []killplan.Kill{{Tick: 960}}},
		{ID: "seg-003", TickStart: 1280, TickEnd: 1920, Kills: []killplan.Kill{{Tick: 1600}}},
	}
	beatTimes := []float64{0.5, 1, 1.5, 2, 2.5}
	analysis := rhythm.Analysis{
		SchemaVersion:     rhythm.SchemaVersion,
		SourceSHA256:      fmt.Sprintf("%x", sha256.Sum256(music)),
		KillOffsetSeconds: 0.1,
		BeatTimesSeconds:  beatTimes,
		SegmentSync:       rhythm.BuildSegmentSyncWithTrim(fullPlan, beatTimes, 0.1, 0),
	}
	body, err := json.Marshal(analysis)
	if err != nil {
		t.Fatal(err)
	}
	rhythmPath := filepath.Join(dir, "rhythm.json")
	if err := os.WriteFile(rhythmPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	prefix := fullPlan
	prefix.Segments = append([]killplan.Segment(nil), fullPlan.Segments[:2]...)
	reordered := prefix
	reordered.Segments = []killplan.Segment{fullPlan.Segments[1], fullPlan.Segments[0]}
	mismatched := prefix
	mismatched.Segments = append([]killplan.Segment(nil), prefix.Segments...)
	mismatched.Segments[1].ID = "seg-other"
	longer := fullPlan
	longer.Segments = append(append([]killplan.Segment(nil), fullPlan.Segments...), killplan.Segment{
		ID:        "seg-004",
		TickStart: 1920,
		TickEnd:   2560,
		Kills:     []killplan.Kill{{Tick: 2240}},
	})

	tests := []struct {
		name    string
		plan    killplan.Plan
		wantErr bool
	}{
		{name: "first N limit prefix", plan: prefix},
		{name: "reordered prefix", plan: reordered, wantErr: true},
		{name: "mismatched prefix segment", plan: mismatched, wantErr: true},
		{name: "render plan longer than analysis", plan: longer, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync, err := loadRhythmSyncForRender(rhythmPath, musicPath, tt.plan, 0)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("loadRhythmSyncForRender error = nil, want ordered-plan mismatch for %#v", tt.plan.Segments)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadRhythmSyncForRender returned error: %v", err)
			}
			if len(sync) != len(prefix.Segments) {
				t.Fatalf("indexed sync len = %d, want render prefix len %d", len(sync), len(prefix.Segments))
			}
		})
	}
}
