package faceit

import "testing"

func TestBestRecentMapPicksTheBestWinRateWithEnoughMatches(t *testing.T) {
	match := func(name, result string) RecentMatch {
		return RecentMatch{Stats: &MatchStats{Map: name, Result: result}}
	}
	matches := []RecentMatch{
		match("de_ancient", "win"),
		match("de_mirage", "win"), match("de_mirage", "win"), match("de_mirage", "loss"),
		match("de_inferno", "win"), match("de_inferno", "loss"),
		match("de_nuke", "win"), match("de_nuke", "unknown"),
		{},
	}
	if got := BestRecentMap(matches); got != "de_mirage" {
		t.Fatalf("BestRecentMap = %q, want de_mirage (one ancient win and one nuke result are too few)", got)
	}
	tie := []RecentMatch{match("de_dust2", "win"), match("de_dust2", "loss"), match("de_anubis", "win"), match("de_anubis", "loss")}
	if got := BestRecentMap(tie); got != "de_anubis" {
		t.Fatalf("tie = %q, want the name order to decide", got)
	}
	if got := BestRecentMap([]RecentMatch{match("de_mirage", "win")}); got != "" {
		t.Fatalf("one match named %q, want no best map", got)
	}
}
