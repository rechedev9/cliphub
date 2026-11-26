package recapplan

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	enemyA = "76561198000000101"
	enemyB = "76561198000000102"
	enemyC = "76561198000000103"
	mate   = "76561198000000201"
)

type cameraKill struct {
	round, tick int
	victim      string
	team        string
	move        float64
	covered     bool
}

func still(round, tick int, victim string) cameraKill {
	return cameraKill{round: round, tick: tick, victim: victim, team: "T", move: 10, covered: true}
}

func cameraFacts(kills ...cameraKill) Facts {
	facts := fixtureFacts()
	facts.Holds = &HoldEvidence{Version: HoldEvidenceVersion, Kills: []KillHold{}}
	for _, k := range kills {
		for i := range facts.Rounds {
			if facts.Rounds[i].Number != k.round {
				continue
			}
			facts.Rounds[i].Kills = append(facts.Rounds[i].Kills, killplan.Kill{Tick: k.tick, Weapon: "ak47",
				Killer: killplan.Player{SteamID64: facts.TargetSteamID64, NameInDemo: "pov", TeamAtKill: "CT"},
				Victim: killplan.Player{SteamID64: k.victim, NameInDemo: "enemy-" + k.victim[max(0, len(k.victim)-3):], TeamAtKill: k.team}})
		}
		facts.Holds.Kills = append(facts.Holds.Kills, KillHold{Tick: k.tick, VictimSteamID64: k.victim, MoveUnits: k.move, Covered: k.covered})
	}
	return facts
}

func cameraOptions(maxWindows int, removed ...string) Options {
	o := fixtureOptions()
	o.SecondCamera = &SecondCameraOptions{Enabled: true, MaxWindows: maxWindows, Removed: removed}
	return o
}

func windowIDs(d Document) []string {
	if d.SecondCamera == nil {
		return nil
	}
	var ids []string
	for _, w := range d.SecondCamera.Windows {
		ids = append(ids, w.ID)
	}
	return ids
}

func hasWarning(d Document, code string) bool {
	return slices.ContainsFunc(d.Warnings, func(n Notice) bool { return n.Code == code })
}

func standardCameraFacts() Facts {
	return cameraFacts(
		still(1, 2900, enemyA),
		cameraKill{round: 1, tick: 6000, victim: enemyB, team: "T", move: 300, covered: true},
		still(1, 8000, enemyC),
		cameraKill{round: 2, tick: 12000, victim: "0", team: "T", move: 0, covered: true},
		cameraKill{round: 2, tick: 12200, victim: mate, team: "CT", move: 0, covered: true},
		cameraKill{round: 2, tick: 12400, victim: enemyA, team: "T", move: 5, covered: false},
		cameraKill{round: 2, tick: 13000, victim: enemyB, team: "T", move: 40, covered: true},
		cameraKill{round: 2, tick: 13500, victim: enemyC, team: "T", move: 20, covered: true},
		cameraKill{round: 25, tick: 17600, victim: enemyA, team: "T", move: 900, covered: true},
		cameraKill{round: 25, tick: 17700, victim: enemyB, team: "T", move: 900, covered: true},
		still(25, 18000, enemyC),
	)
}

func TestSecondCameraOffLeavesPlansAndFactsAlone(t *testing.T) {
	if DefaultOptions().SecondCamera != nil {
		t.Fatal("the second camera must be off by default")
	}
	withHolds := standardCameraFacts()
	without := withHolds
	without.Holds = nil
	a, errA := withHolds.Hash()
	b, errB := without.Hash()
	if errA != nil || errB != nil || a != b {
		t.Fatal("the facts hash must ignore the hold evidence")
	}
	if old, err := HashValue(without); err != nil || old != b {
		t.Fatal("facts without holds must hash as they did before")
	}
	plain := hookPlan(t, without, fixtureOptions())
	off := hookPlan(t, withHolds, fixtureOptions())
	if off.PlanHash != plain.PlanHash || off.SecondCamera != nil {
		t.Fatalf("hold evidence changed a plan without the second camera")
	}
	raw, err := json.Marshal(off)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "second_camera") {
		t.Fatalf("a plan without the second camera carries its key: %s", raw)
	}
	disabled := DefaultOptions()
	disabled.SecondCamera = &SecondCameraOptions{}
	if canonical, err := CanonicalNewOptions(disabled); err != nil || canonical.SecondCamera != nil {
		t.Fatalf("a second camera that is off must leave the canonical options: %+v %v", canonical.SecondCamera, err)
	}
}

func TestSecondCameraPicksStillEnemiesOnePerRound(t *testing.T) {
	d := hookPlan(t, standardCameraFacts(), cameraOptions(5))
	want := []string{"round-001-camera-8000", "round-002-camera-13500", "round-025-camera-18000"}
	if got := windowIDs(d); !reflect.DeepEqual(got, want) {
		t.Fatalf("windows = %v, want %v", got, want)
	}
	w := d.SecondCamera.Windows[0]
	if w.RoundID != "round-001" || w.Round != 1 || w.StartTick != 7500 || w.EndTick != 8100 || w.EnemySteamID64 != enemyC || w.EnemyName != "enemy-103" || w.MoveUnits != 10 {
		t.Fatalf("window = %+v", w)
	}
	if hasWarning(d, WarnSecondCameraSkipped) || hasWarning(d, WarnSecondCameraUnavailable) {
		t.Fatalf("unexpected notices: %+v", d.Warnings)
	}
	again := hookPlan(t, standardCameraFacts(), cameraOptions(5))
	if !reflect.DeepEqual(again.SecondCamera, d.SecondCamera) || again.PlanHash != d.PlanHash {
		t.Fatal("planning the same second camera twice gave different windows")
	}
	plain := hookPlan(t, standardCameraFacts(), fixtureOptions())
	a, errA := plain.CaptureHash()
	b, errB := d.CaptureHash()
	if errA != nil || errB != nil || a != b {
		t.Fatal("the second camera must not change the capture hash")
	}
	if plain.PlanHash == d.PlanHash {
		t.Fatal("the second camera must change the plan hash")
	}
}

func TestSecondCameraIgnoresKillsByOtherPlayers(t *testing.T) {
	facts := cameraFacts(still(1, 8000, enemyC))
	for i := range facts.Rounds {
		for j := range facts.Rounds[i].Kills {
			if facts.Rounds[i].Kills[j].Tick == 8000 {
				facts.Rounds[i].Kills[j].Killer.SteamID64 = mate
			}
		}
	}
	d := hookPlan(t, facts, cameraOptions(5))
	if got := windowIDs(d); got != nil {
		t.Fatalf("a teammate's kill became a second camera window: %v", got)
	}
	if !hasWarning(d, WarnSecondCameraSkipped) {
		t.Fatalf("expected the skipped notice, got %+v", d.Warnings)
	}
}

func TestSecondCameraCapFollowsThePlayScoreAndRemovalPullsNothingIn(t *testing.T) {
	all := hookPlan(t, standardCameraFacts(), cameraOptions(5)).SecondCamera.Windows
	if !(all[1].Score > all[2].Score && all[2].Score > all[0].Score) {
		t.Fatalf("the five-kill play of round 2 must outrank the triple of round 25 and the single of round 1: %+v", all)
	}
	d := hookPlan(t, standardCameraFacts(), cameraOptions(2))
	if got := windowIDs(d); !reflect.DeepEqual(got, []string{"round-002-camera-13500", "round-025-camera-18000"}) {
		t.Fatalf("the cap must keep the best plays in kill order: %v", got)
	}
	removed := hookPlan(t, standardCameraFacts(), cameraOptions(1, "round-002-camera-13500"))
	if removed.SecondCamera != nil || hasWarning(removed, WarnSecondCameraSkipped) {
		t.Fatalf("a removed moment pulled in another one or warned: %v %+v", windowIDs(removed), removed.Warnings)
	}
	two := hookPlan(t, standardCameraFacts(), cameraOptions(5, "round-002-camera-13500"))
	if got := windowIDs(two); !reflect.DeepEqual(got, []string{"round-001-camera-8000", "round-025-camera-18000"}) {
		t.Fatalf("removing round 2 must leave the others: %v", got)
	}
}

func TestSecondCameraNotices(t *testing.T) {
	old := standardCameraFacts()
	old.Holds = nil
	d := hookPlan(t, old, cameraOptions(5))
	if d.SecondCamera != nil || !hasWarning(d, WarnSecondCameraUnavailable) {
		t.Fatalf("facts parsed before holds must ask for a new analysis: %+v", d.Warnings)
	}
	none := hookPlan(t, cameraFacts(still(1, 2900, enemyA)), cameraOptions(5))
	if none.SecondCamera != nil || !hasWarning(none, WarnSecondCameraSkipped) {
		t.Fatalf("no candidate must warn that the camera is skipped: %+v", none.Warnings)
	}
}

func TestSecondCameraOptionsCanonicalAndValidated(t *testing.T) {
	o := DefaultOptions()
	o.SecondCamera = &SecondCameraOptions{Enabled: true, Removed: []string{"round-025-camera-18000", "round-001-camera-8000", "round-025-camera-18000"}}
	canonical, err := CanonicalNewOptions(o)
	if err != nil {
		t.Fatal(err)
	}
	want := &SecondCameraOptions{Enabled: true, MaxWindows: SecondCameraDefaultMax, Removed: []string{"round-001-camera-8000", "round-025-camera-18000"}}
	if !reflect.DeepEqual(canonical.SecondCamera, want) {
		t.Fatalf("canonical = %+v", canonical.SecondCamera)
	}
	if err := canonical.ValidateCurrentFullDemoPolicy(); err != nil {
		t.Fatalf("a plan with the second camera must be admitted: %v", err)
	}
	for name, bad := range map[string]Options{
		"cap":        cameraOptions(SecondCameraMaxWindows + 1),
		"negative":   cameraOptions(-1),
		"removed id": cameraOptions(5, "../round-001"),
		"too many":   cameraOptions(5, slices.Repeat([]string{"round-001-camera-8000"}, SecondCameraMaxRemoved+1)...),
	} {
		if bad.Validate() == nil {
			t.Errorf("%s: invalid second camera options were accepted", name)
		}
	}
}

func TestSecondCameraDocumentRoundTripAndValidation(t *testing.T) {
	d := hookPlan(t, standardCameraFacts(), cameraOptions(5))
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Document
	if err := decodeStrict(raw, &decoded); err != nil || !reflect.DeepEqual(decoded.SecondCamera, d.SecondCamera) {
		t.Fatalf("second camera round trip: %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("a decoded plan with the second camera is invalid: %v", err)
	}
	for name, change := range map[string]func(*Document){
		"two in one round": func(d *Document) {
			w := d.SecondCamera.Windows[1]
			w.KillTick, w.StartTick, w.EndTick, w.EnemySteamID64 = 13000, 12500, 13100, enemyB
			w.ID = CameraWindowID(w.RoundID, w.KillTick)
			d.SecondCamera.Windows = []CameraWindow{d.SecondCamera.Windows[0], w, d.SecondCamera.Windows[1], d.SecondCamera.Windows[2]}
		},
		"removed": func(d *Document) { d.Options.SecondCamera.Removed = []string{d.SecondCamera.Windows[0].ID} },
		"outside the round": func(d *Document) {
			w := &d.SecondCamera.Windows[0]
			w.KillTick, w.StartTick, w.EndTick = 2900, 2400, 3000
			w.ID = CameraWindowID(w.RoundID, w.KillTick)
		},
		"not a kill": func(d *Document) { d.SecondCamera.Windows[0].EnemySteamID64 = enemyA },
		"off":        func(d *Document) { d.Options.SecondCamera = nil },
		"over cap":   func(d *Document) { d.Options.SecondCamera.MaxWindows = 2 },
	} {
		bad := hookPlan(t, standardCameraFacts(), cameraOptions(5))
		change(&bad)
		bad.PlanHash, _ = bad.Hash()
		if bad.Validate() == nil {
			t.Errorf("%s: an invalid second camera was accepted", name)
		}
	}
}

func TestWithCameraEvidenceDropsUnusableWindows(t *testing.T) {
	d := hookPlan(t, standardCameraFacts(), cameraOptions(5))
	same, err := WithCameraEvidence(d, map[string]bool{"round-001-camera-8000": true, "round-002-camera-13500": true, "round-025-camera-18000": true})
	if err != nil || same.PlanHash != d.PlanHash || len(same.Warnings) != len(d.Warnings) {
		t.Fatalf("usable windows changed the document: %v", err)
	}
	effective, err := WithCameraEvidence(d, map[string]bool{"round-025-camera-18000": true})
	if err != nil {
		t.Fatal(err)
	}
	if got := windowIDs(effective); !reflect.DeepEqual(got, []string{"round-025-camera-18000"}) {
		t.Fatalf("effective windows = %v", got)
	}
	var omitted []Notice
	for _, n := range effective.Warnings {
		if n.Code == WarnSecondCameraOmitted {
			omitted = append(omitted, n)
		}
	}
	if len(omitted) != 2 || omitted[0].RoundID != "round-001" || !strings.Contains(omitted[0].Message, "ronda 1 (enemy-103)") {
		t.Fatalf("omitted notices = %+v", omitted)
	}
	if hash, err := effective.Hash(); err != nil || hash != effective.PlanHash || effective.PlanHash == d.PlanHash {
		t.Fatal("the effective copy must carry its own hash")
	}
	if len(d.SecondCamera.Windows) != 3 || len(d.Warnings) == len(effective.Warnings) {
		t.Fatal("the approved document must stay untouched")
	}
	none, err := WithCameraEvidence(d, nil)
	if err != nil || none.SecondCamera != nil {
		t.Fatalf("no usable window must clear the second camera: %v", err)
	}
}

func TestSecondCameraNeverFilmsTheKillThatEndsTheMatch(t *testing.T) {
	d := hookPlan(t, cameraFacts(still(1, 8450, enemyA), still(25, 18950, enemyB)), cameraOptions(5))
	if got, want := windowIDs(d), []string{"round-001-camera-8450"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("windows = %v, want %v: a window over the last round's end leaves CS2 on the match end screen", got, want)
	}
	early := hookPlan(t, cameraFacts(still(25, 18700, enemyB)), cameraOptions(5))
	if got, want := windowIDs(early), []string{"round-025-camera-18700"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("windows = %v, want %v: an earlier kill of the last round still gets its window", got, want)
	}
	short := cameraFacts(still(25, 18700, enemyB))
	short.EndTick = 18900
	short.Rounds[2].RoundEndTick = 18900
	if got := windowIDs(hookPlan(t, short, cameraOptions(5))); got != nil {
		t.Fatalf("windows = %v, want none: the demo ends too soon after the window to rewind", got)
	}
}
