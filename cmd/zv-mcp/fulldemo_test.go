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
	"github.com/rechedev9/cliphub/internal/tuiclient"
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
// plan as stale. Skew either way must ask for a restart; a real stale plan must not.
func TestDecodeFullDemoDocument(t *testing.T) {
	facts := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198000000001",
		ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 2000, Complete: true,
		Rounds: []recapplan.RoundFacts{{ID: "round-001", Number: 1, StartTick: 100, FreezeEndTick: 400, RoundEndTick: 1000, Evidence: "round-events",
			Kills: []killplan.Kill{{Tick: 600, Weapon: "ak47", ClutchVs: 2}}, Utility: []killplan.UtilityThrow{}}}}
	options := recapplan.DefaultOptions()
	options.Audio.Music.Enabled, options.Audio.Voice.Enabled, options.Editorial.KeepFreezeVoice = false, false, false
	doc, err := recapplan.Plan(facts, options, recapplan.VoiceEvidence{Availability: "not_requested", Activity: []recapplan.TickRange{}}, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeFullDemoDocument(raw)
	if err != nil {
		t.Fatalf("Studio's own plan: %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Studio's own plan does not validate: %v", err)
	}

	edit := func(change func(wire map[string]any)) []byte {
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		change(wire)
		b, err := json.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	round := func(wire map[string]any) map[string]any { return wire["rounds"].([]any)[0].(map[string]any) }
	skewed := map[string][]byte{
		"newer Studio": edit(func(wire map[string]any) {
			round(wire)["kills"].([]any)[0].(map[string]any)["field_from_a_newer_studio"] = 2
		}),
		"older Studio": edit(func(wire map[string]any) { delete(round(wire), "live_end_tick") }),
	}
	for name, b := range skewed {
		if _, err := decodeFullDemoDocument(b); !errors.Is(err, errOutdatedMCP) || !strings.Contains(err.Error(), "restart the assistant") {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	stale, err := decodeFullDemoDocument(edit(func(wire map[string]any) { wire["plan_hash"] = strings.Repeat("0", 64) }))
	if err != nil {
		t.Fatalf("a stale plan must decode: %v", err)
	}
	if err := stale.Validate(); err == nil || !strings.Contains(err.Error(), recapplan.ErrPlanStale) {
		t.Errorf("stale plan: err = %v", err)
	}
}

// The defaults are strict recapplan.Options too, so a newer Studio's option must
// ask for a restart as well instead of an unreadable-state error.
func TestLoadFullDemoStateSkewedDefaults(t *testing.T) {
	defaults, err := json.Marshal(recapplan.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	newer := strings.Replace(string(defaults), "{", `{"option_from_a_newer_studio":true,`, 1)
	for name, body := range map[string]string{"same Studio": string(defaults), "newer Studio": newer} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"document":null,"compatibility":"legacy-until-planned-and-approved","defaults":` + body + `}`))
		}))
		c := tuiclient.New(tuiclient.Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
		state, err := loadFullDemoState(t.Context(), c, "e16560b4-6ee2-48a6-8e40-15e5d712856d")
		srv.Close()
		if name == "same Studio" && (err != nil || state.Document != nil || state.Defaults.Overlays.HUDTheme != "arena") {
			t.Errorf("%s: state = %+v, err = %v", name, state, err)
		}
		if name == "newer Studio" && !errors.Is(err, errOutdatedMCP) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
