package tacticalplan

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func firstDeathZones(p FirstDeathPlayer) map[string]float64 {
	out := map[string]float64{}
	for _, z := range p.Zones {
		out[z.Callout] = z.Seconds
	}
	return out
}

func firstDeathPlayer(t *testing.T, fd FirstDeathPositions, steamID string) FirstDeathPlayer {
	t.Helper()
	for _, p := range fd.Players {
		if p.SteamID64 == steamID {
			return p
		}
	}
	t.Fatalf("player %s missing from %+v", steamID, fd.Players)
	return FirstDeathPlayer{}
}

func TestFirstDeathSecondsPerZoneAndMovement(t *testing.T) {
	m := pbMatch(t)
	filter := ScoutFilter{Maps: []string{"de_mirage"}, Side: SideT}
	window := PrepWindow{FromSeconds: 0, ToSeconds: 15}
	fd := BuildFirstDeathPositions([]FramedMatch{m}, FirstDeathOptions{Filter: filter, Window: window, Movement: MovementStatic})
	if fd.RoundCount != 7 || fd.Framed != 6 || len(fd.Players) != 2 {
		t.Fatalf("first death = %d rounds, %d framed, players %+v", fd.RoundCount, fd.Framed, fd.Players)
	}
	two := firstDeathPlayer(t, fd, "steam2")
	// Six framed T rounds: 33 s at the T spawn and 19 s in Middle, the first sample of each round has no speed.
	if two.Rounds != 6 || !reflect.DeepEqual(firstDeathZones(two), map[string]float64{"TSpawn": 5.5, "Middle": 3.17}) {
		t.Fatalf("steam2 = %d rounds, zones %v", two.Rounds, firstDeathZones(two))
	}
	if z := two.Zones[0]; z.Callout != "TSpawn" || z.Rounds != 4 || len(z.Refs) != 4 || len(z.Samples) != 66 {
		t.Errorf("T spawn = %s, %d rounds, %d refs, %d samples", z.Callout, z.Rounds, len(z.Refs), len(z.Samples))
	}
	fast := BuildFirstDeathPositions([]FramedMatch{m}, FirstDeathOptions{Filter: filter, Window: window, Movement: MovementFast})
	// The jump from the spawn to Middle 5 s in is the only fast sample of the window.
	if got := firstDeathZones(firstDeathPlayer(t, fast, "steam2")); !reflect.DeepEqual(got, map[string]float64{"Middle": 0.17}) {
		t.Errorf("fast zones = %v", got)
	}
}

func TestFirstDeathStopsAtTheRoundsFirstDeath(t *testing.T) {
	m := pbMatch(t)
	m.Document.Rounds = slices.Clone(m.Document.Rounds)
	m.Document.Rounds[0].Class.OpeningTick = pbTick(1, 5)
	fd := BuildFirstDeathPositions([]FramedMatch{m}, FirstDeathOptions{
		Filter: ScoutFilter{Maps: []string{"de_mirage"}, Side: SideT}, Window: PrepWindow{FromSeconds: 0, ToSeconds: 15}, Movement: MovementStatic,
	})
	if got := firstDeathZones(firstDeathPlayer(t, fd, "steam3"))["TSpawn"]; got != 4.67 {
		t.Errorf("T spawn = %v s per round, want 28 s over 6 rounds once round 1 stops 5 s in", got)
	}
}

func TestParseMovement(t *testing.T) {
	if m, err := ParseMovement(""); err != nil || m != MovementStatic {
		t.Errorf("empty = %q, %v", m, err)
	}
	if _, err := ParseMovement("crouch"); err == nil {
		t.Error("crouch parsed")
	}
}

func TestDataWarnings(t *testing.T) {
	base := pbMatch(t).ScoutMatch
	usual := base
	usual.PlayedAt = time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	older := base
	older.PlayedAt = usual.PlayedAt.AddDate(0, 0, -7)
	stand := base
	stand.PlayedAt = usual.PlayedAt.AddDate(0, 0, -3)
	stand.Document.Players = slices.Clone(base.Document.Players)
	stand.Document.Players[3].SteamID64 = "steam9"
	matches := []ScoutMatch{usual, stand, older}

	filter := ScoutFilter{Maps: []string{"de_mirage"}}
	// 2 a.m. sixteen calendar days later is under 16 x 24 hours: the age counts days, not hours.
	now := time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC)
	got := DataWarnings(DataWarningInput{Newest: usual.PlayedAt, Compared: matches, Kept: []ScoutMatch{usual}, Filter: filter, Now: now})
	at := usual.PlayedAt
	want := []DataWarning{
		{Kind: DataWarningStale, Days: 16, LastPlayedAt: &at},
		{Kind: DataWarningLineup, Changed: 1, Matches: 3},
		{Kind: DataWarningSmallSample, Rounds: 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warnings = %+v, want %+v", got, want)
	}
	filter.Lineup = Lineup{"steam2", "steam3"}
	big := slices.Repeat([]ScoutMatch{usual}, 4)
	fresh := DataWarnings(DataWarningInput{Newest: usual.PlayedAt, Compared: matches, Kept: big, Filter: filter, Now: usual.PlayedAt.AddDate(0, 0, 14)})
	if !reflect.DeepEqual(fresh, []DataWarning{{Kind: DataWarningLineup, Changed: 1, Matches: 3}}) {
		t.Errorf("fresh warnings = %+v, want only the stand-in against the lineup", fresh)
	}
	if none := DataWarnings(DataWarningInput{Now: usual.PlayedAt}); len(none) != 0 {
		t.Errorf("no matches = %+v", none)
	}
}
