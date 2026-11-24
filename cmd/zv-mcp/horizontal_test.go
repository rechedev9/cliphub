package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/overlaylang"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestGenerateShortRebuildsTheHorizontalDraft(t *testing.T) {
	start, end, game := 1.0, 5.0, 0.4
	short := renderplan.EditRequest{Format: renderplan.FormatShort9x16, KillEffect: renderplan.KillEffectGlitch, Transition: renderplan.TransitionWhip,
		CoverStrategy: renderplan.CoverStrategyNone, Intro: true, IntroText: "ACE", Outro: true, HookText: true, PlayBadges: true, WinProbability: true,
		PlayerLabels: true, VictimReplay: true, VictimReplaySpeed: 1, PreRollSeconds: 5, VerticalHUD: true, StatsCard: true,
		KeyDropStyle: "operator", KeyDropCode: "MYCODE", KeyDropStartSeconds: &start, KeyDropEndSeconds: &end, OverlayLanguage: overlaylang.English}
	for _, capture := range []renderplan.ShortCapture{{HUDMode: "deathnotices", PortraitSafeKillfeed: true}, {HUDMode: "gameplay", PortraitSafeKillfeed: true}, {HUDMode: "clean"}} {
		edit, dropped := renderplan.HorizontalEdit(short, capture)
		draft := decodeDraft(t, map[string]any{"preset": "gameplay-pov-60", "segment_ids": []string{"seg-002", "seg-001"}, "edit": edit, "dropped": dropped,
			"music": map[string]any{"key": "song-1", "volume": 0.6, "game_volume": game}, "capture": map[string]any{"needed": false}})

		args := horizontalArguments("job-1", draft)
		if args.Preset != "gameplay-pov-60" || !slices.Equal(args.SegmentIDs, []string{"seg-002", "seg-001"}) {
			t.Fatalf("%+v: preset %s and plays %v, want the 16:9 preset and the Short's plays in order", capture, args.Preset, args.SegmentIDs)
		}
		body, err := shortLook{Preset: args.Preset, shortChoices: args.shortChoices}.resolve()
		if err != nil {
			t.Fatalf("%+v: generate_short refuses the draft: %v", capture, err)
		}
		if got := decodeEdit(t, body.Edit); !sameEdit(got, edit) {
			t.Errorf("%+v: generate_short sends\n%+v\nwant the draft\n%+v", capture, got, edit)
		}
		if m := body.Music; m == nil || m.Key != "song-1" || m.Volume != 0.6 || m.GameVolume == nil || *m.GameVolume != game {
			t.Errorf("%+v: music = %+v, want the Short's track and levels", capture, m)
		}
	}
}

func TestHorizontalDraftNamesWhatTheSixteenByNineLeavesOut(t *testing.T) {
	draft := decodeDraft(t, map[string]any{"preset": "gameplay-pov-60", "segment_ids": []string{"seg-001"}, "dropped": []string{"vertical_hud", "stats_card"},
		"edit":    map[string]any{"format": "landscape-16x9", "killEffect": "clean", "transition": "cut", "capture_hud": "deathnotices"},
		"capture": map[string]any{"needed": true, "missing_segment_ids": []string{"seg-001"}}})
	out := describeHorizontalDraft("job-1", draft)
	if !out.NeedsCapture || !slices.Equal(out.MissingSegmentIDs, []string{"seg-001"}) {
		t.Fatalf("capture = %v %v, want seg-001 recorded again", out.NeedsCapture, out.MissingSegmentIDs)
	}
	for _, want := range []string{"HUD vertical", "Tarjeta de estadísticas", "Killfeed", "CS2"} {
		if !strings.Contains(out.Summary, want) {
			t.Errorf("summary %q misses %q", out.Summary, want)
		}
	}
}

func decodeDraft(t *testing.T, body map[string]any) tuiclient.HorizontalDraft {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var draft tuiclient.HorizontalDraft
	if err := json.Unmarshal(raw, &draft); err != nil {
		t.Fatal(err)
	}
	return draft
}

func sameEdit(a, b renderplan.EditRequest) bool {
	x, errX := json.Marshal(renderplan.NormalizeEditRequest(a))
	y, errY := json.Marshal(renderplan.NormalizeEditRequest(b))
	return errX == nil && errY == nil && string(x) == string(y)
}
