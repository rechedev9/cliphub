package workers

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestWriteFullDemoOverlayDrawsTheHUDDesignInThePlanColours(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	putJSON(t, store, artifacts.RosterKey(id), parser.RosterResult{
		Players: []parser.PlayerStat{{SteamID64: "76561198000000001", Name: "pov", Team: "CT", Kills: 21, Deaths: 12, Rounds: 20}},
		Match:   parser.MatchInfo{Map: "de_mirage", ScoreCT: 13, ScoreT: 7},
	})
	plan := minimalKillPlan()
	j := job.Job{ID: id, TargetSteamID: "76561198000000001", KillPlan: &plan, Rules: rules.Default()}
	w := NewRenderWorker(newFakeRepo(), store, RenderWorkerConfig{WorkDir: t.TempDir()})
	options := recapplan.DefaultOptions()
	options.Overlays.Theme, options.Overlays.HUDTheme = recapplan.OverlayThemeHUD, "ember"
	options.Overlays.HUDCustom = &hudthemes.Style{Name: "Mío", CT: "112233", T: "445566", Background: "0A0B0C"}
	edit := renderplan.FullDemoEditRequest(recapplan.Snapshot{Document: recapplan.Document{Options: options}})
	path, err := w.writeFullDemoOverlay(j, t.TempDir(), "gameplay-pov-60", edit)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc demooverlay.Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Theme != demooverlay.ThemeHUD || doc.Palette == nil || doc.Palette.CT != "112233" || doc.Palette.T != "445566" || doc.Palette.Background != "0A0B0C" {
		t.Fatalf("overlay design = %q %+v, want the HUD design in the own theme's colours", doc.Theme, doc.Palette)
	}
}
