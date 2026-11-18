package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/playercard"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

const rendererFlag = "--cliphub-render-overlay"

// TestMain doubles as Studio's overlay renderer: the editor runs this binary with
// --cliphub-render-overlay, and it answers with a blank PNG of the asked size.
func TestMain(m *testing.M) {
	if i := slices.Index(os.Args, rendererFlag); i >= 0 && i+1 < len(os.Args) {
		if err := fakeOverlayRender(os.Args[i+1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeOverlayRender(requestPath string) error {
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		return err
	}
	var req struct {
		HTMLPath   string `json:"html_path"`
		OutputPath string `json:"output_path"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return err
	}
	if _, err := os.Stat(req.HTMLPath); err != nil {
		return err
	}
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewNRGBA(image.Rect(0, 0, req.Width, req.Height))); err != nil {
		return err
	}
	return os.WriteFile(req.OutputPath, body.Bytes(), 0o600)
}

// cardShort is the top moments of the fixture as a one-player 9:16 Short that opens and closes on a
// player card and takes a third one as its cover: the editor config without its output folder.
type cardShort struct {
	cfg     editor.Config
	recPlan recording.RecordingPlan
	name    string
}

func newCardShort(t *testing.T, work string) cardShort {
	t.Helper()
	t.Setenv("LocalAppData", filepath.Join(work, "cache"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(work, "cache"))
	fixture := readFixture(t)
	plan := buildKillPlan(t, fixture)
	ranked := moments.Rank(plan)
	ids := make([]string, 0, topMoments)
	for _, moment := range ranked[:min(topMoments, len(ranked))] {
		ids = append(ids, moment.SegmentID)
	}
	selected, err := killplan.Select(plan, ids)
	if err != nil {
		t.Fatalf("select top moments: %v", err)
	}
	selectedPath := filepath.Join(work, "selected-plan.json")
	writeJSON(t, selectedPath, selected)
	recPlan, err := recording.NewPlanFromKillPlan(selected, filepath.Join(work, "match.dem"), filepath.Join(work, "recording"), recording.DefaultStreamConfig())
	if err != nil {
		t.Fatalf("recording plan: %v", err)
	}
	rating, adr := 1.31, 92.4
	cardsPath := filepath.Join(work, "render-data", "player-cards.json")
	writeJSON(t, cardsPath, playercard.Request{Opening: "carta", End: "mvp", Cover: "ficha", Card: playercard.Card{
		SteamID64: fixture.Target.SteamID64, Name: fixture.Target.NameInDemo, Map: fixture.Map,
		Result: &playercard.Result{Own: 13, Rival: 9}, Rounds: 22, Kills: 21, Deaths: 14, Rating: &rating, ADR: &adr,
	}})
	return cardShort{recPlan: recPlan, name: fixture.Target.NameInDemo, cfg: editor.Config{
		KillPlanPath: selectedPath, PublishDir: filepath.Join(work, "publish"), FFmpegPath: "ffmpeg", FFprobePath: "ffprobe",
		OutputFormat: editor.OutputFormatShort9x16, CompileSegments: true, SegmentIDs: ids, PlayerCardsPath: cardsPath, DryRun: true,
	}}
}

// TestPlayerCardShortPipelineGolden walks a one-player 9:16 Short that opens and
// closes on a player card and takes a third one as its cover.
func TestPlayerCardShortPipelineGolden(t *testing.T) {
	work := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZV_OVERLAY_RENDERER_PATH", self)
	t.Setenv("ZV_OVERLAY_RENDERER_APP", "")
	short := newCardShort(t, work)
	recPlan := short.recPlan

	outDir := filepath.Join(work, "render")
	cfg := short.cfg
	cfg.RecordingResultPath, cfg.OutputDir, cfg.Intro = writeRecordingResult(t, recPlan), outDir, true
	if _, err := editor.Run(context.Background(), cfg); err != nil {
		t.Fatalf("editor dry run: %v", err)
	}
	for _, card := range []string{"carta", "mvp", "ficha"} {
		if _, err := os.Stat(filepath.Join(outDir, "player-card-"+card+".png")); err != nil {
			t.Fatalf("the %s card was not drawn: %v", card, err)
		}
	}
	var manifest editor.Manifest
	readJSON(t, filepath.Join(outDir, "edit-manifest.json"), &manifest)
	manifestGolden := normalizedJSON(t, work, manifest)
	var normalized editor.Manifest
	if err := json.Unmarshal([]byte(manifestGolden), &normalized); err != nil {
		t.Fatalf("decode normalized manifest: %v", err)
	}
	assertGolden(t, "20-player-cards-manifest.json", manifestGolden)
	assertGolden(t, "20-player-cards-ffmpeg-commands.txt", formatCommands(normalized)+formatCoverCommands(normalized))

	// The opening card covers the frame, so a phrase said under it shows once the card is gone.
	var phrases []subtitles.Phrase
	for i, segment := range recPlan.Segments {
		start := recording.EffectiveRecordStartTick(segment, recPlan.Tickrate)
		phrases = append(phrases, subtitles.Phrase{ID: strconv.Itoa(i), Speaker: short.name, Text: "vamos", StartTick: start, EndTick: start + 3*recPlan.Tickrate})
	}
	phrasesPath := filepath.Join(work, "subtitles.json")
	writeJSON(t, phrasesPath, subtitles.Track{Phrases: phrases})
	plainDir := filepath.Join(work, "render-subtitles")
	cfg = short.cfg
	cfg.RecordingResultPath, cfg.OutputDir, cfg.SubtitlesPath = writeRecordingResult(t, recPlan), plainDir, phrasesPath
	if _, err := editor.Run(context.Background(), cfg); err != nil {
		t.Fatalf("editor dry run with subtitles: %v", err)
	}
	var withSubtitles editor.Manifest
	readJSON(t, filepath.Join(plainDir, "edit-manifest.json"), &withSubtitles)
	edited := withSubtitles.Shorts[0]
	card := slices.IndexFunc(edited.Effects, func(e editor.Effect) bool { return e.Source == "player-card-opening" })
	if card < 0 || edited.SubtitlesASS == "" {
		t.Fatalf("the Short lacks its opening card (%d) or its subtitles (%q)", card, edited.SubtitlesASS)
	}
	if first := firstCueSeconds(t, edited.SubtitlesASS); first < edited.Effects[card].EndSeconds {
		t.Fatalf("a subtitle starts at %.2f s, under the opening card that ends at %.2f s", first, edited.Effects[card].EndSeconds)
	}
}

// A card the overlay renderer cannot draw leaves the Short without its cards, with a warning, instead of
// failing it.
func TestAPlayerCardThatCannotBeDrawnLeavesTheShortWithoutIt(t *testing.T) {
	work := t.TempDir()
	t.Setenv("ZV_OVERLAY_RENDERER_PATH", "")
	short := newCardShort(t, work)
	cfg := short.cfg
	cfg.RecordingResultPath, cfg.OutputDir = writeRecordingResult(t, short.recPlan), filepath.Join(work, "render")
	result, err := editor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("the Short failed on its player card: %v", err)
	}
	if !slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.Contains(w, "player card") }) {
		t.Fatalf("warnings = %q, want the card that was left out", result.Warnings)
	}
	var manifest editor.Manifest
	readJSON(t, filepath.Join(cfg.OutputDir, "edit-manifest.json"), &manifest)
	for _, s := range manifest.Shorts {
		if slices.ContainsFunc(s.Effects, func(e editor.Effect) bool { return strings.HasPrefix(e.Source, "player-card") }) {
			t.Fatalf("Short %s still shows a card it could not draw", s.SegmentID)
		}
	}
}

// firstCueSeconds reads when the first Dialogue line of an ASS script starts.
func firstCueSeconds(t *testing.T, path string) float64 {
	t.Helper()
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(string(script)) {
		fields := strings.Split(strings.TrimPrefix(line, "Dialogue: "), ",")
		if !strings.HasPrefix(line, "Dialogue: ") || len(fields) < 2 {
			continue
		}
		var h, m int
		var s float64
		if _, err := fmt.Sscanf(fields[1], "%d:%d:%f", &h, &m, &s); err != nil {
			t.Fatalf("read cue start %q: %v", fields[1], err)
		}
		return float64(h*3600+m*60) + s
	}
	t.Fatal("the subtitles have no cue")
	return 0
}

// formatCoverCommands prints each cover command the way formatCommands prints the render.
func formatCoverCommands(manifest editor.Manifest) string {
	var covers editor.Manifest
	for _, short := range manifest.Shorts {
		short.FFmpegCommand = short.CoverCommand
		short.SegmentID += " cover"
		covers.Shorts = append(covers.Shorts, short)
	}
	return formatCommands(covers)
}
