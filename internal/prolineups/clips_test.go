package prolineups

import (
	"strings"
	"testing"
)

func TestClipNameIsStableAndPathSafe(t *testing.T) {
	e := Example{Match: "2398992", MapKey: "2398992/PariVision-vs-Magic-m1-mirage(2).dem", Round: 7, Tick: 40123, SteamID64: "76561198000984547"}
	if got := ClipName(e); got != "2398992/parivision-vs-magic-m1-mirage-2--r7-t40123-76561198000984547.mp4" {
		t.Fatalf("clip name %q", got)
	}
	for _, bad := range []Example{
		{Match: "../x", MapKey: "../x/a.dem", Round: 1, Tick: 1, SteamID64: "76561198000984547"},
		{Match: "1", MapKey: "1/a.dem", Round: 1, Tick: 1},
		{Match: "1", MapKey: "2/a.dem", Round: 1, Tick: 1, SteamID64: "76561198000984547"},
	} {
		if got := ClipName(bad); got != "" {
			t.Fatalf("unsafe example %+v named %q", bad, got)
		}
	}
}

func TestAttachClipsMarksRecordedExamplesAndTheLineupClip(t *testing.T) {
	a := Example{Match: "1", MapKey: "1/a-m1-mirage.dem", Round: 3, Tick: 100, SteamID64: "76561198000000001"}
	b := Example{Match: "2", MapKey: "2/b-m1-mirage.dem", Round: 4, Tick: 200, SteamID64: "76561198000000002"}
	idx := MapIndex{Lineups: []Lineup{{ID: "x", Examples: []Example{a, b}}, {ID: "y", Examples: []Example{a}}}}
	recorded := map[string]bool{ClipName(b): true}
	AttachClips(&idx, func(name string) bool { return recorded[name] })
	x := idx.Lineups[0]
	if x.Examples[0].Clip != "" || x.Examples[1].Clip != ClipName(b) || x.Clip != ClipName(b) {
		t.Fatalf("lineup x = %+v", x)
	}
	if idx.Lineups[1].Clip != "" {
		t.Fatalf("lineup y has no recorded example but got %q", idx.Lineups[1].Clip)
	}
}

func TestClipWindowCoversTheWindUpAndTheFlight(t *testing.T) {
	start, end := ClipWindow(10000, 64, 1.5)
	if start != 10000-3*64 || end != 10000+int(3.5*64) {
		t.Fatalf("window %d-%d", start, end)
	}
}

func TestValidClipNameMatchesClipName(t *testing.T) {
	e := Example{Match: "1", MapKey: "1/a-m1-mirage.dem", Round: 3, Tick: 100, SteamID64: "76561198000000001"}
	if !ValidClipName(ClipName(e)) {
		t.Fatalf("%q must be valid", ClipName(e))
	}
	for _, bad := range []string{"", "1/../a-r1-t1-76561198000000001.mp4", "x/a-r1-t1-76561198000000001.mp4", "1/a.mp4", "1/a-r1-t1-1.mp4"} {
		if ValidClipName(bad) {
			t.Errorf("%q must be invalid", bad)
		}
	}
}

func TestComboExamplesListEveryThrowAndGetTheirOwnClip(t *testing.T) {
	throws := []Throw{}
	for i, team := range []string{"x", "y", "z", "x"} {
		mapKey := "1" + string(rune('0'+i)) + "/a-m1-mirage.dem"
		for _, p := range []string{"76561198000000001", "76561198000000002"} {
			a := smoke(mapKey, p, 1, [3]float64{0, 0, 0}, [3]float64{1000, 0, 0}, 10, 5)
			b := smoke(mapKey, p, 1, [3]float64{500, 500, 0}, [3]float64{0, 1500, 0}, 90, 5)
			a.Match, b.Match = mapKey[:2], mapKey[:2]
			a.Team, b.Team = team, team
			a.ThrowTick, b.ThrowTick = 1000, 1400
			b.Clock = 8
			if p == "76561198000000002" {
				a.Round, b.Round = 2, 2
			}
			throws = append(throws, a, b)
		}
	}
	idx := BuildIndex("de_mirage", throws, now)
	if len(idx.Combos) != 1 {
		t.Fatalf("combos = %d", len(idx.Combos))
	}
	e := idx.Combos[0].Examples[0]
	if len(e.Throws) != 2 || e.Throws[0].Tick != 1000 || e.Throws[1].Tick != 1400 || e.Throws[1].SteamID64 != e.SteamID64 || e.Throws[0].Lineup == e.Throws[1].Lineup {
		t.Fatalf("combo example throws = %+v", e.Throws)
	}
	name := ComboClipName(e)
	if !ValidClipName(name) || name == ClipName(e) || !strings.HasSuffix(name, "-combo.mp4") {
		t.Fatalf("combo clip name %q", name)
	}
	AttachClips(&idx, func(n string) bool { return n == name })
	if idx.Combos[0].Clip != name || idx.Combos[0].Examples[0].Clip != name {
		t.Fatalf("combo clip not attached: %+v", idx.Combos[0])
	}
}
