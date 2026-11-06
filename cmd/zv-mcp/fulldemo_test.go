package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func ptrTo[T any](v T) *T { return &v }

func TestApplyFullDemoChoicesCustomHUD(t *testing.T) {
	got, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{
		HUD: " Prism ", OverlaySource: "FACEIT", TeamVoices: ptrTo(false), VoiceLevel: ptrTo(150), GameLevel: ptrTo(80),
		BetweenRoundEffects: ptrTo(false), TrueView: ptrTo(true), Rounds: []int{5, 3, 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Overlays.HUDTheme != "prism" || got.Capture.HUDProfile != customhud.CaptureProfile {
		t.Errorf("hud = %q / %q", got.Overlays.HUDTheme, got.Capture.HUDProfile)
	}
	if got.SourceKind != "faceit" || got.Audio.Voice.Enabled || got.Audio.Voice.Gain != 1.5 || got.Audio.Game.Gain != 0.8 {
		t.Errorf("source or audio not applied: %+v", got.Audio)
	}
	if got.Transitions == nil || got.Transitions.Enabled || !got.Capture.TrueView || !slices.Equal(got.Rounds, []int{3, 5}) {
		t.Errorf("effects, true view or rounds not applied")
	}
}

func TestApplyFullDemoChoicesNativeHUD(t *testing.T) {
	for _, choice := range []string{"native", "Original de CS2", "cs2"} {
		got, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{HUD: choice})
		if err != nil {
			t.Fatalf("%q: %v", choice, err)
		}
		if got.Overlays.HUDTheme != "" || got.Capture.HUDProfile != recapplan.NativeHUDProfile {
			t.Errorf("%q: hud = %q / %q", choice, got.Overlays.HUDTheme, got.Capture.HUDProfile)
		}
	}
}

func TestApplyFullDemoChoicesDefaultsStay(t *testing.T) {
	got, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Overlays.HUDTheme != "arena" || got.SourceKind != "demo" || !got.Audio.Voice.Enabled || got.Audio.Voice.Gain != 1.1 || got.Transitions == nil || !got.Transitions.Enabled {
		t.Errorf("defaults changed: %+v", got)
	}
}

func TestApplyFullDemoChoicesRejects(t *testing.T) {
	cases := map[string]planFullDemoIn{
		"unknown hud":     {HUD: "neon"},
		"unknown source":  {OverlaySource: "twitch"},
		"voice too loud":  {VoiceLevel: ptrTo(201)},
		"negative game":   {GameLevel: ptrTo(-1)},
		"round zero":      {Rounds: []int{0}},
		"round too large": {Rounds: []int{1000}},
	}
	for name, in := range cases {
		if _, err := applyFullDemoChoices(recapplan.DefaultOptions(), in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	_, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{HUD: "neon"})
	if err == nil || !strings.Contains(err.Error(), "arena") || !strings.Contains(err.Error(), nativeHUDID) {
		t.Errorf("error should list the valid designs: %v", err)
	}
}

func TestDescribeFullDemoPlan(t *testing.T) {
	opts := recapplan.DefaultOptions()
	doc := recapplan.Document{
		PlanHash: "abc123", Options: opts, Clock: recapplan.Clock{FPS: 60},
		Rounds:   make([]recapplan.Round, 3),
		Timeline: []recapplan.TimelineItem{{EndFrame: 60 * 600}, {EndFrame: 60 * 1800}},
		Warnings: []recapplan.Notice{{Message: "a\x00b"}},
		Blockers: []recapplan.Notice{{Message: "asset missing"}},
	}
	out := describeFullDemoPlan("job-1", doc)
	if out.DurationSeconds != 1800 || out.Rounds != 3 || out.HUD != "arena" || out.VoiceLevel != 110 || out.GameLevel != 100 || !out.BetweenRoundEffects {
		t.Errorf("unexpected plan: %+v", out)
	}
	if out.Ready || len(out.Blockers) != 1 || out.Warnings[0] != "ab" {
		t.Errorf("blockers or warnings wrong: %+v", out)
	}
	if !strings.Contains(out.Summary, "30 min") || !strings.Contains(out.Summary, "Arena") || out.PlanHash != "abc123" || !strings.Contains(out.Summary, "3 rondas") {
		t.Errorf("summary = %q", out.Summary)
	}
	doc.Options.Overlays.HUDTheme = ""
	if got := describeFullDemoPlan("job-1", doc); got.HUD != nativeHUDID || !strings.Contains(got.Summary, nativeHUDName) {
		t.Errorf("native plan = %+v", got)
	}
}

func TestFetchPreview(t *testing.T) {
	webp := append([]byte("RIFF\x04\x00\x00\x00WEBP"), "data"...)
	mux := http.NewServeMux()
	mux.HandleFunc("/hud/ok.webp", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(webp) })
	mux.HandleFunc("/hud/html.webp", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>not an image</html>")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if got, err := fetchPreview(context.Background(), srv.URL, "ok"); err != nil || string(got) != string(webp) {
		t.Errorf("valid preview: %v", err)
	}
	for _, id := range []string{"html", "missing"} {
		if _, err := fetchPreview(context.Background(), srv.URL, id); err == nil {
			t.Errorf("%s: accepted", id)
		}
	}
}

// A zv-mcp older than Studio once dropped kills[].clutch_vs and reported every
// plan as stale; an unknown field must say the assistant needs a restart.
func TestDecodeFullDemoDocument(t *testing.T) {
	doc := recapplan.Document{Options: recapplan.DefaultOptions(), Clock: recapplan.Clock{FPS: 60},
		Rounds: []recapplan.Round{{ID: "r1", Kills: []killplan.Kill{{Tick: 100, Weapon: "ak47"}}}}}
	hash, err := doc.Hash()
	if err != nil {
		t.Fatal(err)
	}
	doc.PlanHash = hash
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeFullDemoDocument(raw)
	if err != nil {
		t.Fatalf("Studio's own plan: %v", err)
	}
	if again, _ := got.Hash(); again != hash {
		t.Fatalf("round trip changed the hash: %s != %s", again, hash)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	kill := wire["rounds"].([]any)[0].(map[string]any)["kills"].([]any)[0].(map[string]any)
	kill["field_from_a_newer_studio"] = 2
	newer, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeFullDemoDocument(newer); !errors.Is(err, errOutdatedMCP) || !strings.Contains(err.Error(), "restart the assistant") {
		t.Fatalf("newer plan: err = %v", err)
	}
}
