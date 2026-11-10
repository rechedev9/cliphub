package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// seedLineupMatch is seedLibraryMatch with the CT team's one player and the day it was played.
func (e libraryTestEnv) seedLineupMatch(sha, steamID string, playedAt time.Time) library.Match {
	e.t.Helper()
	ctx := context.Background()
	d := library.Demo{ID: uuid.New(), SHA256: sha, FileName: sha + ".dem", PlayedAt: &playedAt}
	d.Path = artifacts.LibraryDemoKey(d.ID)
	if _, err := e.repo.CreateDemo(ctx, &d); err != nil {
		e.t.Fatal(err)
	}
	doc, blob := tacticalFixture(e.t, uuid.Nil)
	doc.Teams = []tacticalplan.Team{
		{Key: "ct-start", Name: "Vitality", StartSide: tacticalplan.SideCT},
		{Key: "t-start", Name: "MOUZ", StartSide: tacticalplan.SideT},
	}
	doc.Players = []tacticalplan.Player{
		{Slot: 0, SteamID64: steamID, Name: "ct", TeamKey: "ct-start", StartSide: tacticalplan.SideCT},
		{Slot: 1, SteamID64: "76561198000000002", Name: "two", TeamKey: "t-start", StartSide: tacticalplan.SideT},
	}
	b, _ := json.Marshal(doc)
	_ = e.storage.Put(artifacts.LibraryTacticalIndexKey(d.ID), bytes.NewReader(b))
	_ = e.storage.Put(artifacts.LibraryTacticalPositionsKey(d.ID), bytes.NewReader(blob.Data))
	m, err := e.repo.SaveAnalysis(ctx, library.AnalysisFromDocument(d.ID, doc))
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func TestLibraryTeamWarningsCompareTheLineupWithTheMatchesBeforeIt(t *testing.T) {
	env := newLibraryTestEnv(t)
	regular, standIn := "76561198000000001", "76561198000000009"
	m := env.seedLineupMatch("regular", regular, time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC))
	newest := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	env.seedLineupMatch("stand-in", standIn, newest)
	base := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String() + "/warnings"

	var resp dataWarningsResponse
	env.json(http.MethodGet, base+"?map=de_mirage&lineup="+regular, nil, http.StatusOK, &resp)
	if len(resp.Warnings) != 3 {
		t.Fatalf("warnings = %+v, want stale, lineup and small sample", resp.Warnings)
	}
	stale, lineup, small := resp.Warnings[0], resp.Warnings[1], resp.Warnings[2]
	// The lineup filter keeps only the regular's match, yet the stand-in's still counts as a change.
	if lineup != (tacticalplan.DataWarning{Kind: tacticalplan.DataWarningLineup, Changed: 1, Matches: 2}) {
		t.Errorf("lineup = %+v", lineup)
	}
	if small != (tacticalplan.DataWarning{Kind: tacticalplan.DataWarningSmallSample, Rounds: 2}) {
		t.Errorf("small sample = %+v, want the regular's two rounds", small)
	}
	// The age is the team's newest match, not the newest one the lineup kept.
	if stale.Kind != tacticalplan.DataWarningStale || stale.LastPlayedAt == nil || !stale.LastPlayedAt.Equal(newest) {
		t.Errorf("stale = %+v, want the stand-in's match of %s", stale, newest)
	}
	if rw := env.do(http.MethodGet, base+"?side=X", nil, ""); rw.Code != http.StatusBadRequest {
		t.Errorf("bad side = %d, want 400", rw.Code)
	}
}
