package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestGetPublishAssistantLongVideoTemplates(t *testing.T) {
	for _, tc := range []struct {
		player string
		kills  int
	}{{"donk", 0}, {"donk", 34}, {"<>", 34}} {
		t.Run(tc.player+strconv.Itoa(tc.kills), func(t *testing.T) {
			kills := tc.kills
			trends := &fakePublishAssistantTrends{}
			h, url := newPublishAssistantFixture(t, trends, publishAssistantFacts{Player: tc.player, Map: "de_mirage", KillCount: kills})
			req := assistantRequest(http.MethodGet, url)
			id := uuid.MustParse(chi.URLParam(req, "id"))
			variant, name := chi.URLParam(req, "variant"), chi.URLParam(req, "name")
			fixture, err := os.ReadFile("../../web/lib/full-demo-plan.fixture.json")
			if err != nil {
				t.Fatal(err)
			}
			evidence := &editor.FullDemoRenderEvidence{}
			if err := json.Unmarshal(fixture, &evidence.Approved); err != nil {
				t.Fatal(err)
			}
			evidence.Effective = evidence.Approved.Document
			evidence.Effective.Options.SourceKind = "faceit"
			evidence.Effective.Voice = recapplan.VoiceEvidence{Availability: "available", SelectedPackets: 10, Activity: []recapplan.TickRange{{Start: 128, End: 200}}}
			evidence.TrackLevels = []editor.FullDemoTrackLevel{{Role: "team-voice", Measurement: editor.LoudnessMeasurement{Status: "measured"}}}
			putAssistantJSON(t, h.storage.(*fakeStorage), mustAssistantRef(t, id, variant, renderplan.RenderVariantArtifactResult, ""), editor.Result{Shorts: []editor.ShortResult{{SegmentID: name, FullDemo: evidence}}})
			rw := httptest.NewRecorder()
			h.GetPublishAssistant(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rw.Code, rw.Body.String())
			}
			var response publishAssistantResponse
			if err := json.Unmarshal(rw.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Recommendations) < 3 || len(response.Recommendations) > 5 || trends.callCount() != 0 {
				t.Fatalf("expected 3..5 factual templates without trends: %+v", response)
			}
			if response.Metadata.Title != response.Recommendations[0].Title {
				t.Fatal("initial draft differs from first template")
			}
			for _, rec := range response.Recommendations {
				for _, value := range append(append([]string{}, rec.Tags...), rec.Keywords...) {
					if strings.TrimSpace(value) == "" {
						t.Fatal("blank metadata would fail the frontend parser")
					}
				}
				if rec.Template == "" || !strings.Contains(rec.Title, "FACEIT") || !strings.Contains(rec.Title, "COMMS") || !strings.Contains(rec.Title, "Mirage") {
					t.Fatalf("missing long-video facts: %+v", rec)
				}
				if containsAnyWord(rec.Title+rec.Description+strings.Join(rec.Tags, " "), "shorts", "ace", "clutch", "elo", "pro", "rank") {
					t.Fatalf("unverified claim: %+v", rec)
				}
			}
			if kills == 34 && !strings.Contains(response.Metadata.Title, "34 KILLS") {
				t.Fatal("missing video kill count")
			}
		})
	}
}

func TestPublishAssistantLongVideoEffectiveEvidence(t *testing.T) {
	evidence := &editor.FullDemoRenderEvidence{Effective: recapplan.Document{Options: recapplan.DefaultOptions()}}
	evidence.Effective.Clock.TickRate = 64
	evidence.Effective.Timeline = []recapplan.TimelineItem{{Role: "round", SourceStartTick: 128, EndSample: 48000}}
	evidence.Approved.Document.Options.SourceKind = "faceit"
	evidence.Effective.Options.Overlays.Source = "faceit" // Enrichment is not demo origin.
	evidence.Effective.Voice = recapplan.VoiceEvidence{Availability: "available", SelectedPackets: 4, Activity: []recapplan.TickRange{{Start: 128, End: 160}}}
	evidence.TrackLevels = []editor.FullDemoTrackLevel{{Role: "team-voice", Measurement: editor.LoudnessMeasurement{Status: "measured"}}}
	for _, mode := range []string{"muted", "disabled", "missing", "silent", "no-packets", "no-track", "cut-speech", "no-activity"} {
		t.Run(mode, func(t *testing.T) {
			copy := *evidence
			switch mode {
			case "muted":
				copy.Effective.Options.Audio.Voice.Gain = 0
			case "disabled":
				copy.Effective.Options.Audio.Voice.Enabled = false
			case "missing":
				copy.Effective.Voice.Availability = "unavailable"
			case "silent":
				copy.TrackLevels = []editor.FullDemoTrackLevel{{Role: "team-voice", Measurement: editor.LoudnessMeasurement{Status: "silent"}}}
			case "no-packets":
				copy.Effective.Voice.SelectedPackets = 0
			case "no-track":
				copy.TrackLevels = nil
			case "cut-speech":
				copy.Effective.Voice.Activity = []recapplan.TickRange{{Start: 0, End: 128}, {Start: 192, End: 256}}
			case "no-activity":
				copy.Effective.Voice.Activity = nil
			}
			facts := publishAssistantFacts{Player: "player", Map: "de_nuke", KillCount: 20}
			addLongVideoPublishFacts(&facts, editor.ShortResult{FullDemo: &copy}, editor.PublishItem{})
			for _, rec := range longVideoPublishRecommendations(facts) {
				if strings.Contains(rec.Title, "COMMS") || strings.Contains(rec.Title, "FACEIT") {
					t.Fatalf("invented source/voice: %s", rec.Title)
				}
			}
		})
	}
}

func TestPublishAssistantVoiceUsesRetainedSampleWindows(t *testing.T) {
	doc := recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Timeline: []recapplan.TimelineItem{
		{Role: "sponsor", SourceStartTick: 0, StartSample: 0, EndSample: 48000},
		{Role: "round", SourceStartTick: 128, SourceOffsetFrames: 60, StartSample: 48000, EndSample: 96000},
	}}
	for _, tc := range []struct {
		name     string
		activity recapplan.TickRange
		want     bool
	}{
		{"sponsor-only", recapplan.TickRange{Start: 0, End: 64}, false},
		{"before-offset", recapplan.TickRange{Start: 128, End: 192}, false},
		{"retained", recapplan.TickRange{Start: 200, End: 220}, true},
		{"after-trim", recapplan.TickRange{Start: 256, End: 300}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc.Voice.Activity = []recapplan.TickRange{tc.activity}
			if got := retainedPublishVoice(doc); got != tc.want {
				t.Fatalf("retained voice = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPublishAssistantLongVideoOpponentAndCache(t *testing.T) {
	evidence := &editor.FullDemoRenderEvidence{Effective: recapplan.Document{Input: recapplan.Input{TargetSteamID64: "target"}, Rounds: []recapplan.Round{{Kills: []killplan.Kill{
		{Killer: killplan.Player{SteamID64: "target", TeamAtKill: "CT"}, Victim: killplan.Player{SteamID64: "friend", TeamAtKill: "CT", NameInDemo: "teammate"}},
		{Killer: killplan.Player{SteamID64: "someone", TeamAtKill: "CT"}, Victim: killplan.Player{SteamID64: "other", TeamAtKill: "T", NameInDemo: "unrelated"}},
		{Killer: killplan.Player{SteamID64: "target", TeamAtKill: "CT"}, Victim: killplan.Player{SteamID64: "enemy", TeamAtKill: "T", NameInDemo: "w0nderful"}},
	}}}}}
	facts := publishAssistantFacts{Player: "s1mple", Map: "Mirage", KillCount: 24}
	addLongVideoPublishFacts(&facts, editor.ShortResult{FullDemo: evidence}, editor.PublishItem{})
	if facts.Opponent != "w0nderful" {
		t.Fatalf("opponent = %q", facts.Opponent)
	}
	recs := longVideoPublishRecommendations(facts)
	if len(recs) != 5 || !strings.Contains(recs[4].Title, "s1mple vs w0nderful") {
		t.Fatalf("templates: %+v", recs)
	}
	id, now := uuid.New(), time.Now()
	key := publishAssistantCacheKey(id, "variant", "name", facts, 7, now)
	for _, field := range []string{"source", "comms", "opponent", "format"} {
		changed := facts
		switch field {
		case "source":
			changed.SourceKind = "faceit"
		case "comms":
			changed.Comms = true
		case "opponent":
			changed.Opponent = "other"
		case "format":
			changed.LongVideo = false
		}
		if key == publishAssistantCacheKey(id, "variant", "name", changed, 7, now) {
			t.Fatalf("cache ignores %s", field)
		}
	}
}

func TestPublishAssistantLongVideoBoundsAndLegacy(t *testing.T) {
	facts := publishAssistantFacts{Player: strings.Repeat("界", 120) + "\n<test>", Map: strings.Repeat("x", 120), Opponent: strings.Repeat("y", 120), KillCount: 34, Comms: true, SourceKind: "faceit"}
	for _, rec := range longVideoPublishRecommendations(facts) {
		if utf8.RuneCountInString(rec.Title) > 100 || strings.ContainsAny(rec.Title, "\n<>") {
			t.Fatalf("invalid title: %q", rec.Title)
		}
	}
	facts = publishAssistantFacts{}
	addLongVideoPublishFacts(&facts, editor.ShortResult{}, editor.PublishItem{SegmentID: "demo-compilation", Preset: "gameplay-pov-60"})
	if !facts.LongVideo || facts.Comms || facts.SourceKind != "" {
		t.Fatalf("unsafe legacy facts: %+v", facts)
	}
}
