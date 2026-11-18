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
	"testing"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/playercard"
	"github.com/rechedev9/cliphub/internal/recording"
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

	outDir := filepath.Join(work, "render")
	if _, err := editor.Run(context.Background(), editor.Config{
		RecordingResultPath: writeRecordingResult(t, recPlan),
		KillPlanPath:        selectedPath,
		OutputDir:           outDir,
		PublishDir:          filepath.Join(work, "publish"),
		FFmpegPath:          "ffmpeg",
		FFprobePath:         "ffprobe",
		OutputFormat:        editor.OutputFormatShort9x16,
		CompileSegments:     true,
		SegmentIDs:          ids,
		Intro:               true,
		PlayerCardsPath:     cardsPath,
		DryRun:              true,
	}); err != nil {
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
