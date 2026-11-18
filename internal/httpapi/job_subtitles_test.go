package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

func TestJobSubtitlesListOnlyThePhrasesOfThePlayersTeam(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	ctx := context.Background()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, TargetSteamID: "76561198000000002"}
	if err := env.jobs.Create(ctx, &j); err != nil {
		t.Fatal(err)
	}
	if err := env.repo.LinkDemoJob(ctx, env.match.DemoID, j.ID); err != nil {
		t.Fatal(err)
	}
	path := "/api/jobs/" + j.ID.String() + "/subtitles"
	var got jobSubtitles
	env.json(http.MethodGet, path, nil, http.StatusOK, &got)
	if got.MatchID != env.match.ID || got.State != library.TranscriptNone || got.Availability != "not_requested" || len(got.Lines) != 0 {
		t.Fatalf("before any transcript = %+v", got)
	}

	env.transcriber.state = teamvoice.TranscriberInstalled
	env.json(http.MethodPost, env.base, nil, http.StatusAccepted, nil)
	env.await(func(d demoTranscript) bool { return d.State == library.TranscriptReady })
	env.json(http.MethodGet, path, nil, http.StatusOK, &got)
	if got.Availability != "available" || len(got.Lines) != 1 || got.Lines[0].SteamID64 != j.TargetSteamID {
		t.Fatalf("transcribed = %+v; want the one phrase of the player's team", got)
	}
	env.json(http.MethodGet, "/api/jobs/"+uuid.NewString()+"/subtitles", nil, http.StatusNotFound, nil)
}
