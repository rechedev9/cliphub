package workers

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestWriteFullDemoOverlayCarriesTheHalftimeScoreOnlyWhenChosenAndKnown(t *testing.T) {
	const pov = "76561198000000001"
	store := newFakeStorage()
	id := uuid.New()
	putJSON(t, store, artifacts.RosterKey(id), parser.RosterResult{
		Players: []parser.PlayerStat{
			{SteamID64: pov, Name: "pov", Team: "T", Kills: 21, Deaths: 12, Rounds: 20},
			{SteamID64: "76561198000000002", Name: "rival", Team: "CT", Kills: 15, Deaths: 18, Rounds: 20},
		},
		Match: parser.MatchInfo{Map: "de_mirage", ScoreCT: 7, ScoreT: 13, ClanNameT: "Vitality", ClanNameCT: "Spirit"},
	})
	rounds := []killplan.RoundResult{}
	for number := 1; number <= 16; number++ {
		side, winner := "CT", "CT"
		if number > 12 {
			side = "T"
		}
		if number > 8 {
			winner = "T"
		}
		rounds = append(rounds, killplan.RoundResult{Number: number, Winner: winner, TargetSide: side})
	}
	halftimeOf := func(t *testing.T, chosen bool, results []killplan.RoundResult) *demooverlay.Halftime {
		t.Helper()
		plan := minimalKillPlan()
		plan.Target.SteamID64, plan.Rounds = pov, results
		j := job.Job{ID: id, TargetSteamID: pov, KillPlan: &plan, Rules: rules.Default()}
		w := NewRenderWorker(newFakeRepo(), store, RenderWorkerConfig{WorkDir: t.TempDir()})
		options := recapplan.DefaultOptions()
		options.Overlays.Halftime = chosen
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
		return doc.Halftime
	}
	got := halftimeOf(t, true, rounds)
	if got == nil || got.Round != 13 {
		t.Fatalf("halftime = %+v, want the side switch at round 13", got)
	}
	if got.Teams[0] != (demooverlay.HalftimeTeam{Name: "Vitality", Side: "T", Score: 8}) || got.Teams[1] != (demooverlay.HalftimeTeam{Name: "Spirit", Side: "CT", Score: 4}) {
		t.Fatalf("halftime teams = %+v, want the POV team first with the demo's names", got.Teams)
	}
	if off := halftimeOf(t, false, rounds); off != nil {
		t.Fatalf("a plan without the halftime card got one: %+v", off)
	}
	if unknown := halftimeOf(t, true, nil); unknown != nil {
		t.Fatalf("a job without round results got a halftime score: %+v", unknown)
	}
}

func TestFullDemoRenderHashFollowsTheStoryOverlays(t *testing.T) {
	hashOf := func(t *testing.T, change func(*recapplan.Options)) string {
		t.Helper()
		result, _, _ := fullDemoPublicationFixture(t, "clip", func(_ *recapplan.Facts, o *recapplan.Options) { change(o) })
		snapshot := recapplan.Snapshot{Document: *result.Plan.FullDemo, Approval: recapplan.Approval{PlanHash: result.Plan.FullDemo.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
		hash, err := fullDemoRenderFingerprint(result, "gameplay-pov-60", snapshot)
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	plain := hashOf(t, func(*recapplan.Options) {})
	if again := hashOf(t, func(*recapplan.Options) {}); again != plain {
		t.Fatal("the fixture's render hash is not stable")
	}
	halftime := hashOf(t, func(o *recapplan.Options) { o.Overlays.Halftime = true })
	subscribe := hashOf(t, func(o *recapplan.Options) { o.Overlays.Subscribe = true })
	if halftime == plain || subscribe == plain || halftime == subscribe {
		t.Fatal("each story overlay must give the render its own identity")
	}
}
