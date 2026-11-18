package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/subtitles"
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

// A long match says more phrases than one page of the voice store; the panel and the video read them all.
func TestJobSubtitlesReadTheWholeTranscriptOfALongMatch(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	ctx := context.Background()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, TargetSteamID: "76561198000000002"}
	if err := env.jobs.Create(ctx, &j); err != nil {
		t.Fatal(err)
	}
	if err := env.repo.LinkDemoJob(ctx, env.match.DemoID, j.ID); err != nil {
		t.Fatal(err)
	}
	team := teamOf(env.match, j.TargetSteamID)
	lines := make([]library.DemoVoiceLine, library.MaxVoiceLineList+1)
	for i := range lines {
		lines[i] = library.DemoVoiceLine{ID: uuid.New(), SteamID64: j.TargetSteamID, Name: "b1t", TeamKey: team, StartMS: int64(i) * 1000, EndMS: int64(i)*1000 + 500, Tick: 64 + i*64, Round: 1 + i/200, Text: "fase"}
	}
	ready := library.DemoVoiceTranscript{State: library.TranscriptReady, English: library.TranscriptNone}
	if err := env.repo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: env.match.ID, Transcript: ready, Lines: &lines}); err != nil {
		t.Fatal(err)
	}
	var got jobSubtitles
	env.json(http.MethodGet, "/api/jobs/"+j.ID.String()+"/subtitles", nil, http.StatusOK, &got)
	if len(got.Lines) != len(lines) || got.Lines[len(got.Lines)-1].ID != lines[len(lines)-1].ID {
		t.Fatalf("the panel lists %d phrases, want all %d up to the last round", len(got.Lines), len(lines))
	}
}

// linkedJob is a parsed job of the env's demo, for the player whose team says the phrases.
func (e demoTranscriptEnv) linkedJob() job.Job {
	e.t.Helper()
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan, TargetSteamID: "76561198000000002"}
	if err := e.jobs.Create(context.Background(), &j); err != nil {
		e.t.Fatal(err)
	}
	if err := e.repo.LinkDemoJob(context.Background(), e.match.DemoID, j.ID); err != nil {
		e.t.Fatal(err)
	}
	return j
}

// A render never holds its slot for a transcription: without a finished transcript it goes on without
// subtitles, and the transcription it queued serves the next render.
func TestSubtitlesNeverHoldTheRender(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	gate := make(chan struct{})
	env.transcriber.gate = gate
	j := env.linkedJob()
	req := subtitles.Request{JobID: j.ID, Target: j.TargetSteamID}
	done := make(chan error, 1)
	go func() {
		_, err := env.h.Subtitles(context.Background(), req)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("subtitles came back before the transcription ran")
		}
	case <-time.After(5 * time.Second):
		close(gate)
		t.Fatal("the render waited for the transcription")
	}
	close(gate)
	env.await(func(d demoTranscript) bool { return d.State == library.TranscriptReady })
	track, err := env.h.Subtitles(context.Background(), req)
	if err != nil || len(track.Phrases) != 1 || track.Phrases[0].Text != "Rotamos a B ya" {
		t.Fatalf("subtitles once transcribed = %+v, %v; want the team's phrase", track, err)
	}
}

// Creating a Short with subtitles starts the transcription, so it runs while CS2 records.
func TestGenerateWithSubtitlesQueuesTheTranscription(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	env.h.capabilities.RecordEnabled = true
	j := env.linkedJob()
	rw := postGenerate(t, env.h, j.ID, `{"preset":"clean-pov-60","edit":{"format":"short-9x16","killEffect":"clean","transition":"cut","voice_comms":true,"subtitles":true}}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("generate = %d %s", rw.Code, rw.Body.String())
	}
	if got := env.await(func(d demoTranscript) bool { return d.State != library.TranscriptNone && !busyTranscript(d.State) }); got.State != library.TranscriptReady {
		t.Fatalf("transcript = %+v, want the one the generate queued", got)
	}
}
