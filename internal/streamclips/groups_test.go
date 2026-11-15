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
	if got := plan.Placements()["clip-3"]; got.Video != 1 || got.Videos != 2 || math.Abs(got.MusicOffsetSeconds-10) > 1e-9 {
		t.Fatalf("second member placement = %+v, want video 1 of 2 at 10 s of music", got)
	}
	if got := plan.Placements()["clip-2"]; got.Video != 2 || got.MusicOffsetSeconds != 0 {
		t.Fatalf("lone moment placement = %+v, want video 2 with the music from its start", got)
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
	if got := plan.Placements()["clip-1"].MusicOffsetSeconds; math.Abs(got-15) > 1e-9 {
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
	cases := map[string]struct {
		mutate func(*EditPlan)
		want   string
	}{
		"one member":   {func(p *EditPlan) { p.Groups[0].ClipIDs = []string{"clip-1"} }, "at least 2"},
		"unknown clip": {func(p *EditPlan) { p.Groups[0].ClipIDs = []string{"clip-1", "clip-9"} }, "unknown clip"},
		"id of a clip": {func(p *EditPlan) { p.Groups[0].ID = "clip-2" }, "already used"},
		"bad id":       {func(p *EditPlan) { p.Groups[0].ID = "../x" }, "invalid group id"},
		"clip in two": {func(p *EditPlan) {
			p.Groups = append(p.Groups, ClipGroup{ID: "group-2", ClipIDs: []string{"clip-1", "clip-2"}})
		}, "in groups group-1 and group-2"},
		"clip twice": {func(p *EditPlan) { p.Groups[0].ClipIDs = []string{"clip-1", "clip-3", "clip-1"} }, "lists clip clip-1 twice"},
		"duplicate groups": {func(p *EditPlan) {
			p.Groups = append(p.Groups, ClipGroup{ID: "group-1", ClipIDs: []string{"clip-2", "clip-2"}})
		}, "already used"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			plan := groupedPlan()
			c.mutate(&plan)
			if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Validate = %v, want an error with %q", err, c.want)
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
