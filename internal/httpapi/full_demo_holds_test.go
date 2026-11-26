package httpapi

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

const holdsFixtureEnemy = "76561198000000101"

func olderParseWithAKill(t *testing.T, store *fakeStorage, j job.Job) string {
	t.Helper()
	facts, found, err := recapplan.LoadFacts(store, j.ID)
	if err != nil || !found {
		t.Fatalf("facts: %t %v", found, err)
	}
	if facts.Holds != nil {
		t.Fatal("the fixture must stand for a parse older than the hold evidence")
	}
	facts.Rounds[1].Kills = []killplan.Kill{{Tick: 15000, Weapon: "ak47",
		Killer: killplan.Player{SteamID64: j.TargetSteamID, NameInDemo: "pov", TeamAtKill: "CT"},
		Victim: killplan.Player{SteamID64: holdsFixtureEnemy, NameInDemo: "rival", TeamAtKill: "T"}}}
	if err := recapplan.StoreFacts(store, j.ID, facts); err != nil {
		t.Fatal(err)
	}
	hash, err := facts.Hash()
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func planWithSecondCamera(t *testing.T, h *Handlers, j job.Job, options recapplan.Options) recapplan.Document {
	t.Helper()
	options.SecondCamera = &recapplan.SecondCameraOptions{Enabled: true}
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if rw.Code != 201 {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body)
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSecondCameraReadsEnemyMovementForAnOlderParse(t *testing.T) {
	h, j, store, _, options := fullDemoAPIFixture(t)
	hashBefore := olderParseWithAKill(t, store, j)
	reads := 0
	h.killHolds.read = func(context.Context, storage.Storage, job.Job) (*recapplan.HoldEvidence, error) {
		reads++
		return &recapplan.HoldEvidence{Version: recapplan.HoldEvidenceVersion, Kills: []recapplan.KillHold{
			{Tick: 15000, VictimSteamID64: holdsFixtureEnemy, MoveUnits: 12, Covered: true},
		}}, nil
	}

	plain := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": options})
	if plain.Code != 201 || reads != 0 {
		t.Fatalf("a plan without the second camera must not read the demo: %d, %d reads", plain.Code, reads)
	}

	d := planWithSecondCamera(t, h, j, options)
	if d.SecondCamera == nil || len(d.SecondCamera.Windows) != 1 || d.SecondCamera.Windows[0].EnemySteamID64 != holdsFixtureEnemy {
		t.Fatalf("second camera = %+v, want the still enemy of round 2", d.SecondCamera)
	}
	if len(d.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", d.Warnings)
	}
	again := planWithSecondCamera(t, h, j, options)
	if reads != 1 || again.PlanHash != d.PlanHash {
		t.Fatalf("the demo is read once and the plan stays the same: %d reads", reads)
	}
	stored, _, err := recapplan.LoadFacts(store, j.ID)
	if err != nil || stored.Holds == nil || len(stored.Holds.Kills) != 1 {
		t.Fatalf("the hold evidence was not kept with the facts: %+v %v", stored.Holds, err)
	}
	hashAfter, err := stored.Hash()
	if err != nil || hashAfter != hashBefore || d.Input.FactsHash != hashBefore {
		t.Fatalf("the facts hash moved, so saved plans of this parse would go stale: %s -> %s", hashBefore, hashAfter)
	}
}

func TestSecondCameraPlansWithoutWindowsWhenTheDemoCannotBeRead(t *testing.T) {
	h, j, store, _, options := fullDemoAPIFixture(t)
	olderParseWithAKill(t, store, j)

	d := planWithSecondCamera(t, h, j, options)
	if d.SecondCamera != nil {
		t.Fatalf("second camera = %+v, want none", d.SecondCamera)
	}
	unavailable := slices.ContainsFunc(d.Warnings, func(n recapplan.Notice) bool { return n.Code == recapplan.WarnSecondCameraUnavailable })
	if !unavailable {
		t.Fatalf("warnings = %+v, want %s", d.Warnings, recapplan.WarnSecondCameraUnavailable)
	}
	stored, _, err := recapplan.LoadFacts(store, j.ID)
	if err != nil || stored.Holds != nil {
		t.Fatalf("a failed read must not write hold evidence: %+v %v", stored.Holds, err)
	}
}
