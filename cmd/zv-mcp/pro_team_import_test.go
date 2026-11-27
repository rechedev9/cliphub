package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type fakeProTeamImporter struct {
	answers []tuiclient.ProTeamImport
	calls   int
}

func (f *fakeProTeamImporter) ImportProTeam(_ context.Context, teamID int) (tuiclient.ProTeamImport, error) {
	if teamID != 9565 {
		return tuiclient.ProTeamImport{}, errors.New("wrong team")
	}
	if f.calls >= len(f.answers) {
		return tuiclient.ProTeamImport{}, errors.New("called past the last batch")
	}
	f.calls++
	return f.answers[f.calls-1], nil
}

func TestImportProTeamLoopsUntilNothingIsLeft(t *testing.T) {
	team := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	f := &fakeProTeamImporter{answers: []tuiclient.ProTeamImport{
		{Name: "Vitality", Days: 120, Total: 9, Held: 4, Imported: 4, Remaining: 5},
		{TeamID: &team, Name: "Vitality", Days: 120, Total: 9, Held: 8, Imported: 4, Remaining: 1},
		{TeamID: &team, Name: "Vitality", Days: 120, Total: 9, Held: 8, Imported: 0, Remaining: 0},
	}}
	out, err := importWholeProTeam(t.Context(), f, 9565)
	if err != nil || f.calls != 3 {
		t.Fatalf("calls = %d err = %v", f.calls, err)
	}
	if out.TeamID != team.String() || out.StudioPath != "/patrones/"+team.String() || out.Imported != 8 || out.Held != 8 || out.Total != 9 {
		t.Fatalf("out = %+v", out)
	}
}

func TestImportProTeamStopsWhenABatchMakesNoProgress(t *testing.T) {
	f := &fakeProTeamImporter{answers: []tuiclient.ProTeamImport{
		{Name: "Vitality", Total: 12, Remaining: 6},
		{Name: "Vitality", Total: 12, Remaining: 6},
		{Name: "Vitality", Total: 12, Remaining: 6},
	}}
	out, err := importWholeProTeam(t.Context(), f, 9565)
	if err != nil || f.calls != 2 || out.TeamID != "" || out.StudioPath != "" {
		t.Fatalf("a batch that leaves as many maps must end the loop: calls %d out %+v err %v", f.calls, out, err)
	}
}
