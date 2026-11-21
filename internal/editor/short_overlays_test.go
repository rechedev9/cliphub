package editor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestPlayBadgeLabelNamesClutchAndMultikill(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		kills []KillCue
		want  string
	}{
		{"single kill", []KillCue{{}}, ""},
		{"double", []KillCue{{}, {}}, "2K"},
		{"ace", []KillCue{{}, {}, {}, {}, {}}, "ACE"},
		{"clutch with unknown result", []KillCue{{ClutchVs: 2}, {ClutchVs: 1}}, "CLUTCH 1V2 · 2K"},
		{"won clutch", []KillCue{{ClutchVs: 3, ClutchResult: killplan.ClutchWon}, {ClutchVs: 2, ClutchResult: killplan.ClutchWon}, {ClutchVs: 1, ClutchResult: killplan.ClutchWon}}, "CLUTCH 1V3 GANADO · 3K"},
		{"lost clutch kill", []KillCue{{ClutchVs: 1, ClutchResult: killplan.ClutchLost}}, "CLUTCH 1V1"},
	}
	for _, tc := range cases {
		if got := playBadgeLabel(tc.kills); got != tc.want {
			t.Errorf("%s: playBadgeLabel = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPlayBadgesFollowEachPlayAndReplaceTheCounterMilestone(t *testing.T) {
	t.Parallel()
	short := ShortEdit{
		OutputFormat: OutputFormatShort9x16, DurationSeconds: 20, KillCounter: true,
		Overlays: ShortOverlays{PlayBadges: true},
		Kills:    []KillCue{{TimeSeconds: 2}, {TimeSeconds: 3}, {TimeSeconds: 12}},
		Parts: []ShortPart{
			{SegmentID: "seg-001", DurationSeconds: 8, Kills: []KillCue{{TimeSeconds: 2}, {TimeSeconds: 3, Tick: 300}}},
			{SegmentID: "seg-001-victim", DurationSeconds: 4, TimelineStartSeconds: 8},
			{SegmentID: "seg-002", DurationSeconds: 8, TimelineStartSeconds: 12, Kills: []KillCue{{TimeSeconds: 0.5, ClutchVs: 1, ClutchResult: killplan.ClutchWon}}},
		},
	}
	badges := generatedPlayBadgeEffects(short)
	if len(badges) != 2 {
		t.Fatalf("badges = %+v, want one per play with a label", badges)
	}
	if badges[0].Value != "2K" || badges[0].StartSeconds != 3.05 || badges[0].SourceKillTick != 300 {
		t.Errorf("first badge = %+v, want 2K right after its last kill", badges[0])
	}
	if badges[1].Value != "CLUTCH 1V1 GANADO" || badges[1].StartSeconds != 12.55 {
		t.Errorf("second badge = %+v, want the clutch on the part's timeline", badges[1])
	}
	for _, effect := range generatedKillCounterEffects(short) {
		if strings.HasSuffix(effect.Value, "K") {
			t.Errorf("kill counter still shows the milestone %q next to the badge", effect.Value)
		}
	}
}

func TestPlayBadgeEndsBeforeTheNextPart(t *testing.T) {
	t.Parallel()
	short := ShortEdit{
		OutputFormat: OutputFormatShort9x16, DurationSeconds: 10,
		Overlays: ShortOverlays{PlayBadges: true},
		Parts: []ShortPart{
			{SegmentID: "seg-001", DurationSeconds: 4, Kills: []KillCue{{TimeSeconds: 2.5}, {TimeSeconds: 3}}},
			{SegmentID: "seg-002", DurationSeconds: 6, TimelineStartSeconds: 4},
		},
	}
	badges := generatedPlayBadgeEffects(short)
	if len(badges) != 1 || badges[0].EndSeconds != 4 {
		t.Fatalf("badges = %+v, want the badge cut at the part end", badges)
	}
}

func TestStatsCardLinesLeaveOutUnknownData(t *testing.T) {
	t.Parallel()
	adr, hs, rating, win := 92.44, 54.2, 1.18, 58.0
	full := StatsCard{Player: "ZywOo", Kills: 24, Deaths: 15, ADR: &adr, HSPct: &hs, Rating: &rating,
		FACEIT: &StatsCardFACEIT{Level: 8, WinPct: &win, BestMap: "de_mirage"}}
	want := []string{"K/D 1.60 (24/15)", "ADR 92.4 · HS 54 %", "RATING DE CLIPHUB 1.18", "FACEIT NIVEL 8 · 58 % DE VICTORIAS", "MEJOR MAPA: MIRAGE"}
	if got := statsCardLines(full); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("full card = %q, want %q", got, want)
	}
	bare := StatsCard{Player: "x", Kills: 3}
	if got := statsCardLines(bare); strings.Join(got, "|") != "K/D 3/0" {
		t.Errorf("bare card = %q, want only the kills and deaths", got)
	}
	for _, line := range statsCardLines(full) {
		if strings.Contains(line, "HLTV") {
			t.Errorf("card line %q names HLTV", line)
		}
	}
}

func TestStatsCardClosesTheShortAfterTheLastKill(t *testing.T) {
	t.Parallel()
	card := &StatsCard{Player: "ZywOo", Kills: 10, Deaths: 5}
	short := ShortEdit{OutputFormat: OutputFormatShort9x16, DurationSeconds: 20, Kills: []KillCue{{TimeSeconds: 16.9}}, Overlays: ShortOverlays{StatsCard: card}}
	effects := generatedStatsCardEffects(short)
	if len(effects) < 3 || effects[0].Type != EffectFlash {
		t.Fatalf("effects = %+v, want a dim backdrop and the card text", effects)
	}
	for _, effect := range effects {
		if effect.StartSeconds != 17.5 || effect.EndSeconds != 20 {
			t.Errorf("effect %q spans %v..%v, want 17.5..20", effect.Value, effect.StartSeconds, effect.EndSeconds)
		}
	}
	short.Kills = []KillCue{{TimeSeconds: 19.5}}
	if start, _, _ := statsCardWindow(short); start != 18.5 {
		t.Errorf("late kill: card starts at %v, want the 1.5 s minimum", start)
	}
	short.OutputFormat = OutputFormatLandscape16x9
	if effects := generatedStatsCardEffects(short); effects != nil {
		t.Errorf("landscape Short drew the card: %+v", effects)
	}
}

func TestVictimReplaySpeedSetsTheReplayLengthAndFilters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	run := func(speed float64) []ShortPart {
		result, err := Run(context.Background(), Config{
			RecordingResultPath: writeRecordingResult(t, dir, replayFixture(dir)),
			OutputDir:           filepath.Join(dir, "shorts"),
			FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
			DryRun:              true,
			CompileSegments:     true,
			VictimReplay:        true,
			VictimReplaySpeed:   speed,
		})
		if err != nil {
			t.Fatalf("Run speed %v: %v", speed, err)
		}
		return result.Shorts[0].Parts
	}
	slow := run(0.25)
	if slow[1].Speed != 0.25 || slow[1].DurationSeconds != 14 {
		t.Fatalf("0.25x replay = %+v, want 14 s", slow[1])
	}
	filter := CompilationFilter(ShortEdit{Parts: slow, Transition: TransitionCut, Tickrate: 64})
	if !strings.Contains(filter, "setpts=PTS/0.250,") || !strings.Contains(filter, "atempo=0.500,atempo=0.500,atrim") {
		t.Errorf("0.25x filter does not slow video and chain atempo:\n%s", filter)
	}
	real := run(1)
	filter = CompilationFilter(ShortEdit{Parts: real, Transition: TransitionCut, Tickrate: 64})
	if real[1].DurationSeconds != 3.5 || strings.Contains(filter, "atempo") || strings.Contains(filter, "setpts=PTS/") {
		t.Errorf("1x replay is slowed: %+v\n%s", real[1], filter)
	}
	if !strings.Contains(filter, ",fade=t=in:st=0:d=0.150:color=white") {
		t.Errorf("1x replay lost its white flash:\n%s", filter)
	}
}

func TestRunRejectsAnUnknownVictimReplaySpeed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := Run(context.Background(), Config{
		RecordingResultPath: writeRecordingResult(t, dir, replayFixture(dir)),
		OutputDir:           filepath.Join(dir, "shorts"),
		DryRun:              true,
		VictimReplay:        true,
		VictimReplaySpeed:   0.75,
	})
	if err == nil || !strings.Contains(err.Error(), "victim replay speed") {
		t.Fatalf("Run error = %v, want the speed rejected", err)
	}
}

func TestEndCardHoldsTheLastFrameInsteadOfCoveringTheLastKill(t *testing.T) {
	t.Parallel()
	short := ShortEdit{
		OutputFormat: OutputFormatShort9x16, OutputFPS: 60, DurationSeconds: 20,
		Kills: []KillCue{{TimeSeconds: 8}, {TimeSeconds: 18.7}},
		Parts: []ShortPart{
			{Input: "a.mp4", DurationSeconds: 10, Kills: []KillCue{{TimeSeconds: 8}}},
			{Input: "b.mp4", DurationSeconds: 10, TimelineStartSeconds: 10, Kills: []KillCue{{TimeSeconds: 8.7}}},
		},
		Overlays: ShortOverlays{PlayerCards: &PlayerCards{End: "end.png"}},
	}
	held := withEndCardHold(short)
	if got := held.Parts[1].HoldAfterSeconds; got != 0.8 {
		t.Fatalf("last part hold = %v, want 0.8 s so the card starts 0.6 s after the 18.7 s kill", got)
	}
	if held.DurationSeconds != 20.8 || short.Parts[1].HoldAfterSeconds != 0 {
		t.Errorf("held duration = %v (input part hold %v), want 20.8 and the input untouched", held.DurationSeconds, short.Parts[1].HoldAfterSeconds)
	}
	if start, end, _ := statsCardWindow(held); start != 19.3 || end != 20.8 {
		t.Errorf("end card spans %v..%v, want 19.3..20.8", start, end)
	}
	filter := CompilationFilter(held)
	if !strings.Contains(filter, "setpts=PTS-STARTPTS,tpad=stop_mode=clone:stop=48[pv1]") || !strings.Contains(filter, "apad=pad_dur=0.800000[pa1]") {
		t.Errorf("compilation filter does not hold the last part for 48 frames:\n%s", filter)
	}
	if strings.Contains(filter, "tpad=stop_mode=clone:stop=48[pv0]") {
		t.Errorf("the first part was held too:\n%s", filter)
	}

	early := short
	early.Kills = []KillCue{{TimeSeconds: 8}, {TimeSeconds: 15}}
	if got := withEndCardHold(early); got.DurationSeconds != 20 || got.Parts[1].HoldAfterSeconds != 0 {
		t.Errorf("a kill well before the end held the Short: %+v", got.Parts[1])
	}
	noCard := short
	noCard.Overlays = ShortOverlays{}
	if got := withEndCardHold(noCard); got.DurationSeconds != 20 {
		t.Errorf("a Short without a closing card was held to %v", got.DurationSeconds)
	}
	stats := short
	stats.Overlays = ShortOverlays{StatsCard: &StatsCard{Player: "ZywOo"}}
	if got := withEndCardHold(stats); got.DurationSeconds != 20.8 {
		t.Errorf("stats card Short duration = %v, want it held to 20.8", got.DurationSeconds)
	}
}
