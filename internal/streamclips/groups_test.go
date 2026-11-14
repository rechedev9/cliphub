package streamclips

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func groupedPlan() EditPlan {
	plan := DefaultEditPlan()
	plan.Clips = []ClipRange{
		{ID: "clip-1", StartSeconds: 10, EndSeconds: 20, Title: "TRIPLE KILL"},
		{ID: "clip-2", StartSeconds: 40, EndSeconds: 50, Title: "suelto"},
		{ID: "clip-3", StartSeconds: 60, EndSeconds: 90, Title: "ACE", Edit: &ClipEdit{Speed: 2}},
	}
	plan.Groups = []ClipGroup{{ID: "group-1", ClipIDs: []string{"clip-1", "clip-3"}}}
	return plan
}

func TestGroupedPlanRendersOneVideoPerGroup(t *testing.T) {
	plan := groupedPlan()
	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	outputs := plan.Outputs()
	if len(outputs) != 2 {
		t.Fatalf("outputs = %d, want 2 (one group and one lone moment)", len(outputs))
	}
	group := outputs[0]
	if group.ID != "group-1" || group.Title != "TRIPLE KILL + ACE" {
		t.Fatalf("group output = %q %q", group.ID, group.Title)
	}
	if len(group.Clips) != 2 || group.Clips[0].ID != "clip-1" || group.Clips[1].ID != "clip-3" {
		t.Fatalf("group clips = %+v", group.Clips)
	}
	if got := group.DurationSeconds(); math.Abs(got-25) > 1e-9 {
		t.Fatalf("group duration = %v, want 25 (10 + 30 at 2x)", got)
	}
	if outputs[1].ID != "clip-2" {
		t.Fatalf("lone output = %q", outputs[1].ID)
	}
	if got := plan.MusicOffsetSeconds("clip-3"); math.Abs(got-10) > 1e-9 {
		t.Fatalf("music offset of the second member = %v, want 10", got)
	}
}

func TestGroupOrderIsTheGroupsNotTheVODs(t *testing.T) {
	plan := groupedPlan()
	plan.Groups[0].ClipIDs = []string{"clip-3", "clip-1"}
	plan.Groups[0].Title = "Ronda 12"
	out := plan.Outputs()[0]
	if out.Title != "Ronda 12" || out.Clips[0].ID != "clip-3" {
		t.Fatalf("reordered group = %q starting with %q", out.Title, out.Clips[0].ID)
	}
	if got := plan.MusicOffsetSeconds("clip-1"); math.Abs(got-15) > 1e-9 {
		t.Fatalf("music offset = %v, want 15", got)
	}
}

func TestGroupsSurviveDecode(t *testing.T) {
	raw, err := json.Marshal(groupedPlan())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEditPlan(raw)
	if err != nil {
		t.Fatalf("DecodeEditPlan: %v", err)
	}
	if len(decoded.Groups) != 1 || strings.Join(decoded.Groups[0].ClipIDs, ",") != "clip-1,clip-3" {
		t.Fatalf("decoded groups = %+v", decoded.Groups)
	}
	ungrouped := groupedPlan()
	ungrouped.Groups = nil
	raw, err = json.Marshal(ungrouped)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "groups") {
		t.Fatalf("a plan without groups keeps its old shape: %s", raw)
	}
}

func TestGroupFingerprintMovesWithGrouping(t *testing.T) {
	grouped, err := EditPlanFingerprint(groupedPlan())
	if err != nil {
		t.Fatal(err)
	}
	plain := groupedPlan()
	plain.Groups = nil
	ungrouped, err := EditPlanFingerprint(plain)
	if err != nil {
		t.Fatal(err)
	}
	if grouped == ungrouped {
		t.Fatal("joining moments must change the render fingerprint")
	}
}

func TestInvalidGroups(t *testing.T) {
	cases := map[string]func(*EditPlan){
		"one member":   func(p *EditPlan) { p.Groups[0].ClipIDs = []string{"clip-1"} },
		"unknown clip": func(p *EditPlan) { p.Groups[0].ClipIDs = []string{"clip-1", "clip-9"} },
		"id of a clip": func(p *EditPlan) { p.Groups[0].ID = "clip-2" },
		"bad id":       func(p *EditPlan) { p.Groups[0].ID = "../x" },
		"clip in two": func(p *EditPlan) {
			p.Groups = append(p.Groups, ClipGroup{ID: "group-2", ClipIDs: []string{"clip-1", "clip-2"}})
		},
		"duplicate groups": func(p *EditPlan) {
			p.Groups = append(p.Groups, ClipGroup{ID: "group-1", ClipIDs: []string{"clip-2", "clip-2"}})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			plan := groupedPlan()
			mutate(&plan)
			if err := plan.Validate(); err == nil {
				t.Fatal("Validate accepted an invalid group")
			}
		})
	}
}

func TestNormalizeSplitsAGroupWhoseMomentWasRemoved(t *testing.T) {
	plan := groupedPlan()
	plan.Clips = plan.Clips[:2]
	plan = NormalizeEditPlan(plan)
	if plan.Groups != nil {
		t.Fatalf("groups = %+v, want none once only one member is left", plan.Groups)
	}
	if got := len(plan.Outputs()); got != 2 {
		t.Fatalf("outputs = %d, want 2 lone moments", got)
	}
}
