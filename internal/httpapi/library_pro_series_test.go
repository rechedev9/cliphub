package httpapi

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
)

func TestProLibraryVetoesFillTheSlotsTheyCan(t *testing.T) {
	teams := []community.ProSidecarTeam{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}
	sidecar := community.ProSidecar{Teams: teams, Maps: []string{"Nuke", "Mirage", "Anubis"},
		Veto: []community.ProSidecarVeto{{Team: "Beta", Action: "pick", Map: "Nuke"}, {Team: "Alpha", Action: "pick", Map: "Mirage"}, {Action: "decider", Map: "Anubis"}}}
	alpha, beta := uuid.New(), uuid.New()
	series := []uuid.UUID{beta, alpha}
	want := []library.VetoStep{{Team: "a", Action: "pick", Map: "de_nuke"}, {Team: "b", Action: "pick", Map: "de_mirage"}, {Action: "decider", Map: "de_anubis"}}
	if got := proLibraryVetoes(community.ProSeriesOf(sidecar), proSlots(series, map[int]uuid.UUID{1: alpha, 2: beta})); !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing series keeps its own slot order: %+v", got)
	}
	if got := proLibraryVetoes(community.ProSeriesOf(sidecar), proSlots(series, map[int]uuid.UUID{1: alpha})); !reflect.DeepEqual(got, want) {
		t.Fatalf("with one team in the library, the other's steps take the other slot: %+v", got)
	}
	decider := []library.VetoStep{{Action: "decider", Map: "de_anubis"}}
	if got := proLibraryVetoes(community.ProSeriesOf(sidecar), proSlots(nil, map[int]uuid.UUID{1: alpha})); !reflect.DeepEqual(got, decider) {
		t.Fatalf("a series without teams keeps only the decider: %+v", got)
	}
	sidecar.Veto = nil
	if got := proLibraryVetoes(community.ProSeriesOf(sidecar), nil); !reflect.DeepEqual(got, decider) {
		t.Fatalf("a sidecar without a veto still names the decider of a BO3: %+v", got)
	}
	if pair := proTeamPair(teams, map[int]uuid.UUID{1: alpha, 2: alpha}); pair != nil {
		t.Fatalf("one library team for both sides is no pair: %v", pair)
	}
}

func TestProSeriesRefreshableOnlyUntilAPersonEdits(t *testing.T) {
	s := library.Series{VetoAuto: true}
	edited := library.ApplySeriesPatch(s, library.SeriesPatch{Vetoes: &[]library.VetoStep{}})
	if !proSeriesRefreshable(s) || edited.VetoAuto || proSeriesRefreshable(library.Series{Vetoes: []library.VetoStep{{Action: "decider", Map: "de_nuke"}}}) {
		t.Fatal("an import's series refreshes; a person's edit or veto stays")
	}
}
