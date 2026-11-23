package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

func TestDemoVoiceTranscriptKeepsTheOriginalAndDiscardsAcrossTranscriptions(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	j := env.linkedJob()
	env.json(http.MethodPost, env.base, nil, http.StatusAccepted, nil)
	first := env.await(func(d demoTranscript) bool { return d.State == library.TranscriptReady })
	var corrected, discarded library.DemoVoiceLine
	for _, l := range first.Lines {
		if l.SteamID64 == j.TargetSteamID {
			discarded = l.DemoVoiceLine
		} else {
			corrected = l.DemoVoiceLine
		}
	}
	correctedPath, discardedPath := env.base+"/lines/"+corrected.ID.String(), env.base+"/lines/"+discarded.ID.String()
	env.json(http.MethodPatch, correctedPath, map[string]any{}, http.StatusBadRequest, nil)
	var got library.DemoVoiceLine
	env.json(http.MethodPatch, correctedPath, map[string]any{"text": "Rotamos a B, ya"}, http.StatusOK, &got)
	if got.Text != "Rotamos a B, ya" || got.OriginalText != "Rotamos a B ya" || !got.Corrected || got.Discarded {
		t.Fatalf("correction = %+v; want the transcriber's text beside it", got)
	}
	env.json(http.MethodPatch, discardedPath, map[string]any{"discarded": true}, http.StatusOK, &got)
	if !got.Discarded || got.Text != "Rotamos a B ya" || got.Corrected {
		t.Fatalf("discard = %+v; want the phrase discarded with its text", got)
	}
	req := subtitles.Request{JobID: j.ID, Target: j.TargetSteamID}
	if track := readyTrack(t)(env.h.Subtitles(context.Background(), req)); len(track.Phrases) != 0 {
		t.Fatalf("subtitles with the team's phrase discarded = %+v; want none drawn", track)
	}

	env.json(http.MethodPost, env.base, map[string]bool{"force": true}, http.StatusAccepted, nil)
	again := env.await(func(d demoTranscript) bool {
		return d.State == library.TranscriptReady && env.transcriber.calls.Load() == 4
	})
	for _, l := range again.Lines {
		switch l.ID {
		case corrected.ID:
			if l.Text != "Rotamos a B, ya" || l.OriginalText != "Rotamos a B ya" || !l.Corrected || l.Discarded {
				t.Fatalf("corrected phrase transcribed again = %+v; want the correction kept", l)
			}
		case discarded.ID:
			if !l.Discarded || l.Corrected {
				t.Fatalf("discarded phrase transcribed again = %+v; want it still discarded", l)
			}
		default:
			t.Fatalf("phrase %s changed id", l.ID)
		}
	}

	env.json(http.MethodPatch, discardedPath, map[string]any{"discarded": false}, http.StatusOK, &got)
	if track := readyTrack(t)(env.h.Subtitles(context.Background(), req)); len(track.Phrases) != 1 || track.Phrases[0].ID != discarded.ID.String() {
		t.Fatalf("subtitles with the phrase restored = %+v; want it drawn again", track)
	}
}
