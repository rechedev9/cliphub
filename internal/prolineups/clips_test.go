package prolineups

import "testing"

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
