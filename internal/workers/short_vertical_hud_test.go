package workers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestShortVerticalHUDReadsTheSharedHUDTelemetry(t *testing.T) {
	t.Parallel()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	demo := []byte("stand-in demo bytes")
	sum := sha256.Sum256(demo)
	sha := hex.EncodeToString(sum[:])
	j := job.Job{ID: uuid.New(), DemoPath: "demos/match.dem"}
	if err := store.Put(j.DemoPath, bytes.NewReader(demo)); err != nil {
		t.Fatal(err)
	}
	state := customhud.Example()
	state.Tick = 900
	timeline := customhud.Timeline{Version: customhud.TelemetryVersion, DemoSHA256: sha, TargetSteamID: customhud.ExampleTarget, Map: "de_mirage", TickRate: 64, EndTick: 3000, Snapshots: []customhud.Snapshot{state}}
	body, err := json.Marshal(timeline)
	if err != nil {
		t.Fatal(err)
	}
	// The key the Full Demo HUD caches its telemetry under: a Short reuses it without parsing the demo.
	key := fmt.Sprintf("jobs/%s/full-demo/hud/%s/%s-%s.json", j.ID, customhud.TelemetryVersion, sha, customhud.ExampleTarget)
	if err := store.Put(key, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	plan := recording.RecordingPlan{DemoSHA256: sha, TargetSteamID64: customhud.ExampleTarget, Tickrate: 64, Segments: []recording.RecordingSegment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 1500, ExactWindow: true},
		{ID: "seg-001" + recording.VictimReplaySuffix, TickStart: 1300, TickEnd: 1500, ExactWindow: true},
	}}
	w := &RenderWorker{storage: store}
	in := shortDataInput{job: j, edit: renderplan.EditRequest{Format: renderplan.FormatShort9x16, VerticalHUD: true}, result: recording.RecordingResult{Plan: plan}, workDir: t.TempDir()}
	path := w.writeShortVerticalHUD(context.Background(), in)
	if path == "" {
		t.Fatal("no vertical HUD track written")
	}
	track, err := customhud.LoadVerticalTrack(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(track.Segments) != 1 || track.Segments[0].ID != "seg-001" {
		t.Fatalf("segments = %+v, want the play without its victim replay", track.Segments)
	}
	if spans := track.Segments[0].Spans; len(spans) != 1 || spans[0].TickStart != 1000 || spans[0].TickEnd != 1500+64 || spans[0].State.Round != 3 {
		t.Fatalf("spans = %+v", spans)
	}
	in.edit.VerticalHUD = false
	if w.writeShortVerticalHUD(context.Background(), in) != "" {
		t.Fatal("a Short without the vertical HUD read the telemetry")
	}
}

func TestShortOverlayArgsCarryTheVerticalHUDDesign(t *testing.T) {
	t.Parallel()
	got := shortOverlayArgs(renderplan.EditRequest{CenteredKillfeed: true, VerticalHUD: true, VerticalHUDTheme: "prism"})
	want := []string{"--centered-killfeed", "--vertical-hud", "--vertical-hud-theme", "prism"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}
