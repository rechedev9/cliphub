package community

import (
	"reflect"
	"testing"
)

func TestProSeriesOfReadsVetoAndPicks(t *testing.T) {
	teams := []ProSidecarTeam{{ID: 8297, Name: "FURIA"}, {ID: 6667, Name: "FaZe"}}
	s := ProSeriesOf(ProSidecar{Teams: teams, Maps: []string{"Ancient", "Dust2", "Cache"}, Format: "bo3", Veto: []ProSidecarVeto{
		{Team: "FaZe", Action: "ban", Map: "Inferno"}, {Team: "FURIA", Action: "ban", Map: "Nuke"},
		{Team: "FURIA", Action: "pick", Map: "Ancient"}, {Team: "FaZe Clan", Action: "pick", Map: "Dust2"},
		{Team: "FaZe", Action: "ban", Map: "Mirage"}, {Team: "Unknown", Action: "ban", Map: "Anubis"},
		{Action: "decider", Map: "Cache"}, {Action: "decider", Map: "Cache"}, {Team: "FURIA", Action: "skip", Map: "Train"},
	}})
	if s.Format != "bo3" || len(s.Veto) != 7 {
		t.Fatalf("series = %+v", s)
	}
	if s.Veto[3] != (ProVetoStep{TeamID: 6667, Team: "FaZe Clan", Action: "pick", Map: "de_dust2"}) || s.Veto[5].TeamID != 0 || s.Veto[6] != (ProVetoStep{Action: "decider", Map: "de_cache"}) {
		t.Fatalf("veto = %+v", s.Veto)
	}
	want := []ProSeriesMap{{Number: 1, Map: "de_ancient", PickedBy: 8297}, {Number: 2, Map: "de_dust2", PickedBy: 6667}, {Number: 3, Map: "de_cache", Decider: true}}
	if !reflect.DeepEqual(s.Maps, want) {
		t.Fatalf("maps = %+v", s.Maps)
	}
}

func TestProSeriesOfGivesUnnamedStepsToTheOtherTeam(t *testing.T) {
	teams := []ProSidecarTeam{{ID: 1, Name: "Natus Vincere"}, {ID: 2, Name: "FURIA"}}
	s := ProSeriesOf(ProSidecar{Teams: teams, Maps: []string{"Nuke"}, Veto: []ProSidecarVeto{
		{Team: "NAVI", Action: "ban", Map: "Nuke"}, {Team: "FURIA", Action: "ban", Map: "Mirage"}, {Team: "NAVI", Action: "pick", Map: "Dust2"},
	}})
	if s.Veto[0].TeamID != 1 || s.Veto[1].TeamID != 2 || s.Veto[2].TeamID != 1 {
		t.Fatalf("NAVI matches no name, so its steps belong to Natus Vincere: %+v", s.Veto)
	}
	if s.Maps[0].PickedBy != 0 {
		t.Fatalf("a banned map has no picker: %+v", s.Maps)
	}
}

func TestProSeriesOfOldSidecarCountsMaps(t *testing.T) {
	for maps, want := range map[int]string{1: "bo1", 2: "bo2", 3: "bo3", 4: "", 5: "bo5"} {
		names := []string{"Mirage", "Nuke", "Ancient", "Dust2", "Train"}[:maps]
		s := ProSeriesOf(ProSidecar{Maps: names})
		if s.Format != want || len(s.Veto) != 0 {
			t.Fatalf("%d maps = %+v, want %s", maps, s, want)
		}
		if last := s.Maps[len(s.Maps)-1]; last.Decider != (maps%2 == 1) {
			t.Fatalf("%d maps: the last map is the decider only in an odd series: %+v", maps, s.Maps)
		}
	}
	if got := ProMapName("Dust2"); got != "de_dust2" {
		t.Fatalf("ProMapName = %q", got)
	}
}
