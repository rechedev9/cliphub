package tacticalplan

import (
	"reflect"
	"testing"
)

func prepPlayer(t *testing.T, p Prep, steamID string) PrepPlayer {
	t.Helper()
	for _, pl := range p.CTPlayers {
		if pl.SteamID64 == steamID {
			return pl
		}
	}
	t.Fatalf("player %s missing from %+v", steamID, p.CTPlayers)
	return PrepPlayer{}
}

// spots lists a window's callouts with their round counts, most held first.
func spots(w PrepPlayerWindow) map[string]int {
	out := map[string]int{}
	for _, s := range w.Spots {
		out[s.Callout] = s.Share.Count
	}
	return out
}

func TestPrepReadsCTPositionsPerWindow(t *testing.T) {
	prep := BuildPrep([]FramedMatch{pbMatch(t)}, PrepOptions{
		Filter: ScoutFilter{Maps: []string{"de_mirage"}}, Roles: map[string]string{"steam2": "awp"},
	})
	if prep.Map != "de_mirage" || prep.RoundCount != 9 || prep.Framed != 2 || len(prep.CTPlayers) != 2 {
		t.Fatalf("prep = map %s, %d rounds, %d framed, players %+v", prep.Map, prep.RoundCount, prep.Framed, prep.CTPlayers)
	}
	two := prepPlayer(t, prep, "steam2")
	if two.Role != "awp" || two.Rounds != 2 || len(two.Windows) != len(PrepWindows) {
		t.Fatalf("steam2 = %+v", two)
	}
	if got := spots(two.Windows[0]); !reflect.DeepEqual(got, map[string]int{"BombsiteA": 1, "Middle": 1}) || two.Windows[0].Rounds != 2 {
		t.Errorf("steam2 0-20 s = %v over %d rounds", got, two.Windows[0].Rounds)
	}
	if two.Windows[0].Spots[0].Share.Pct != 50 {
		t.Errorf("steam2 share = %+v, want half the rounds", two.Windows[0].Spots[0].Share)
	}
	three := prepPlayer(t, prep, "steam3")
	if three.Role != "unknown" || three.Rounds != 1 {
		t.Fatalf("steam3 = %+v", three)
	}
	want := []map[string]int{{"Middle": 1}, {"BombsiteB": 1}, {"BombsiteB": 1}, {"BombsiteB": 1}}
	for i, w := range three.Windows {
		if got := spots(w); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("steam3 window %d = %v, want %v", i, got, want[i])
		}
	}
	if ex := three.Windows[1].Spots[0].Examples; len(ex) != 1 || ex[0].Tick != pbTick(8, 25) {
		t.Errorf("steam3 20-40 s example = %+v, want the first frame on B", ex)
	}
	if refs := three.Windows[1].Spots[0].Refs; len(refs) != 1 || refs[0].Tick != pbTick(8, 25) {
		t.Errorf("steam3 20-40 s refs = %+v, want the round on B", refs)
	}
}

func TestPrepMatchupsAndTempoPerBuy(t *testing.T) {
	prep := BuildPrep([]FramedMatch{pbMatch(t)}, PrepOptions{Filter: ScoutFilter{Maps: []string{"de_mirage"}}})
	type row struct {
		side        Side
		buy, rival  BuyType
		rounds, won int
	}
	var got []row
	for _, m := range prep.Matchups {
		got = append(got, row{m.Side, m.Buy, m.OpponentBuy, m.Rounds, m.WinRate.Count})
	}
	want := []row{{SideT, BuyEco, BuyFull, 1, 0}, {SideT, BuyFull, BuyFull, 6, 4}, {SideCT, BuyFull, BuyFull, 2, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matchups = %+v, want %+v", got, want)
	}
	if prep.Matchups[1].Share.Total != 7 {
		t.Errorf("T full share = %+v, want over the 7 T rounds", prep.Matchups[1].Share)
	}
	if refs := prep.Matchups[1].Refs; len(refs) != 6 || len(prep.Matchups[1].Examples) > ScoutExampleLimit {
		t.Errorf("T full refs = %d, want every one of the 6 rounds", len(refs))
	}
	if len(prep.Tempo) != 3 {
		t.Fatalf("tempo rows = %+v", prep.Tempo)
	}
	full := prep.Tempo[1]
	if full.Side != SideT || full.Buy != BuyFull || full.Tempo.Samples != 6 || full.Plant.Samples != 2 || full.Tempo.Median != 50 {
		t.Errorf("T full tempo = %+v", full)
	}
	if ct := prep.Tempo[2]; ct.Side != SideCT || ct.Tempo.Samples != 1 || ct.Tempo.Median != 30 {
		t.Errorf("CT tempo = %+v, want the first contact", ct)
	}
}
