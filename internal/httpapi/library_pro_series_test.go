package httpapi

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
)

func TestProLibraryVetoesKeepOnlyTheDeciderWithoutBothSlots(t *testing.T) {
	teams := []community.ProSidecarTeam{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}
	sidecar := community.ProSidecar{Teams: teams, Maps: []string{"Nuke", "Mirage", "Anubis"},
		Veto: []community.ProSidecarVeto{{Team: "Beta", Action: "pick", Map: "Nuke"}, {Team: "Alpha", Action: "pick", Map: "Mirage"}, {Action: "decider", Map: "Anubis"}}}
	alpha, beta := uuid.New(), uuid.New()
	ids := map[int]uuid.UUID{1: alpha, 2: beta}
	series := []uuid.UUID{beta, alpha}
	got := proLibraryVetoes(community.ProSeriesOf(sidecar), proSlots(series, ids))
	want := []library.VetoStep{{Team: "a", Action: "pick", Map: "de_nuke"}, {Team: "b", Action: "pick", Map: "de_mirage"}, {Action: "decider", Map: "de_anubis"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing series keeps its own slot order: %+v", got)
	}
	decider := []library.VetoStep{{Action: "decider", Map: "de_anubis"}}
	if got := proLibraryVetoes(community.ProSeriesOf(sidecar), proSlots(series, map[int]uuid.UUID{1: alpha})); !reflect.DeepEqual(got, decider) {
		t.Fatalf("a team outside the library leaves only the decider: %+v", got)
	}
	sidecar.Veto = nil
	if got := proLibraryVetoes(community.ProSeriesOf(sidecar), nil); !reflect.DeepEqual(got, decider) {
		t.Fatalf("a sidecar without a veto still names the decider of a BO3: %+v", got)
	}
	if pair := proTeamPair(teams, map[int]uuid.UUID{1: alpha, 2: alpha}); pair != nil {
		t.Fatalf("one library team for both sides is no pair: %v", pair)
	}
}
