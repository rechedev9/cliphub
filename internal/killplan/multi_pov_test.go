package killplan

import (
	"slices"
	"testing"
)

func povPlan(steamID string, starts ...int) Plan {
	p := NewPlan()
	p.Demo = Demo{SHA256: "sha", Tickrate: 64}
	p.Target = Target{SteamID64: steamID, NameInDemo: "p" + steamID}
	for i, start := range starts {
		p.Segments = append(p.Segments, Segment{ID: FormatSegmentID(i + 1), TickStart: start, TickEnd: start + 300})
	}
	p.Stats = Stats{TotalKillsTarget: len(starts), SegmentsCreated: len(starts)}
	return p
}

func TestMergePOVsOrdersRenumbersAndTagsPlayers(t *testing.T) {
	merged, err := MergePOVs([]Plan{povPlan("1", 5000, 20000), povPlan("2", 1000, 20000)})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range merged.Segments {
		got = append(got, s.ID+"@"+s.POV.SteamID64)
	}
	// Equal starts keep pick order: the anchor's play comes first.
	want := []string{"seg-001@2", "seg-002@1", "seg-003@1", "seg-004@2"}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %v, want %v", got, want)
	}
	if merged.Target.SteamID64 != "1" || !merged.IsMultiPOV() {
		t.Fatalf("anchor %q multi=%v, want anchor 1 and multi", merged.Target.SteamID64, merged.IsMultiPOV())
	}
	if merged.Stats.TotalKillsTarget != 4 || merged.Stats.SegmentsCreated != 4 {
		t.Fatalf("stats = %+v, want 4 kills and 4 segments", merged.Stats)
	}
}

func TestMergePOVsRejectsBadInput(t *testing.T) {
	other := povPlan("2", 1000)
	other.Demo.SHA256 = "other"
	for name, plans := range map[string][]Plan{
		"one player":  {povPlan("1", 1000)},
		"other demo":  {povPlan("1", 1000), other},
		"same player": {povPlan("1", 1000), povPlan("1", 9000)},
	} {
		if _, err := MergePOVs(plans); err == nil {
			t.Errorf("%s: merge succeeded, want an error", name)
		}
	}
}

func TestPOVClashNeedsHalfASecondBetweenPlayers(t *testing.T) {
	p := povPlan("1")
	one, two := &Target{SteamID64: "1"}, &Target{SteamID64: "2"}
	a := Segment{TickStart: 1000, TickEnd: 1300, POV: one}
	cases := []struct {
		name string
		b    Segment
		want bool
	}{
		{"overlap", Segment{TickStart: 1200, TickEnd: 1500, POV: two}, true},
		{"half a second later", Segment{TickStart: 1332, TickEnd: 1600, POV: two}, false},
		{"just under half a second", Segment{TickStart: 1331, TickEnd: 1600, POV: two}, true},
		{"just under half a second before", Segment{TickStart: 600, TickEnd: 969, POV: two}, true},
		{"same player overlapping", Segment{TickStart: 1100, TickEnd: 1200, POV: one}, false},
		{"plan target overlapping", Segment{TickStart: 1100, TickEnd: 1200}, false},
	}
	for _, c := range cases {
		if got := p.POVClash(a, c.b); got != c.want {
			t.Errorf("%s: clash = %v, want %v", c.name, got, c.want)
		}
	}
}
