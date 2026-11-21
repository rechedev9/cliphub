package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

func streamIDOf(base string) uuid.UUID {
	return uuid.MustParse(base[len("/api/stream-jobs/"):])
}

var momentoStretch = []library.VoiceSpan{{StartMS: 62_000, EndMS: 75_000}}

func TestAStreamRenderWaitsForTheTranscripcionOfItsStretches(t *testing.T) {
	env := newStreamTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	base := env.stream(micStream)
	req := streamclips.SubtitleRequest{JobID: streamIDOf(base), Stretches: momentoStretch}

	if off := env.h.StreamSubtitles(context.Background(), req); len(off.Lines) != 0 || off.Notice != "" || len(env.transcriber.clips()) != 0 {
		t.Fatalf("with subtítulos off the render got %+v and %d transcriptions; want nothing", off, len(env.transcriber.clips()))
	}

	env.json(http.MethodPatch, base+"/subtitles", map[string]bool{"shorts": true}, http.StatusOK, nil)
	env.transcriber.hold = make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(env.transcriber.hold)
	}()
	got := env.h.StreamSubtitles(context.Background(), req)
	if got.Notice != "" || len(got.Lines) != 1 || got.Lines[0].Text != "vaya tiro" || len(got.Lines[0].Words) != 2 {
		t.Fatalf("render subtitles = %+v; want the frase of the stretch, with its words, once the transcripción finished", got)
	}
	if summary := env.h.StreamSubtitles(context.Background(), streamclips.SubtitleRequest{JobID: req.JobID, Summary: true, Stretches: momentoStretch}); len(summary.Lines) != 0 || summary.Notice != "" {
		t.Fatalf("the Resumen got %+v; want nothing while its own switch is off", summary)
	}

	hide := map[string]any{"video": "clip-1", "line_id": got.Lines[0].ID, "hidden": true}
	env.json(http.MethodPatch, base+"/subtitles", hide, http.StatusOK, nil)
	again := env.h.StreamSubtitles(context.Background(), req)
	if len(env.transcriber.clips()) != 1 || len(again.Hidden["clip-1"]) != 1 || again.Hidden["clip-1"][0] != got.Lines[0].ID {
		t.Fatalf("a second render ran %d transcriptions and got hidden %+v; want the stored frases and the hidden one", len(env.transcriber.clips()), again.Hidden)
	}
}

func TestAStreamRenderGoesWithoutSubtitulosAndSaysWhy(t *testing.T) {
	stretch := streamclips.SubtitleRequest{Stretches: momentoStretch}
	cases := []struct {
		name    string
		probe   streamclips.SourceProbe
		arrange func(env streamTranscriptEnv)
		summary bool
		want    string
	}{
		{name: "the limit passes", probe: micStream, want: "Los vídeos han salido sin subtítulos: la transcripción no terminó a tiempo. Expórtalos de nuevo cuando termine.",
			arrange: func(env streamTranscriptEnv) {
				env.transcriber.state, env.transcriber.hold = teamvoice.TranscriberInstalled, make(chan struct{})
				env.h.voice.streamWait = 80 * time.Millisecond
			}},
		{name: "the transcripción fails", probe: micStream, want: "Los vídeos han salido sin subtítulos: la transcripción falló en este PC.",
			arrange: func(env streamTranscriptEnv) {
				env.transcriber.state, env.transcriber.err = teamvoice.TranscriberInstalled, errors.New("boom")
			}},
		{name: "no transcriber", probe: micStream, summary: true, want: "El resumen ha salido sin subtítulos: la transcripción no está instalada en este PC.",
			arrange: func(streamTranscriptEnv) {}},
		{name: "a mixed track", probe: streamclips.SourceProbe{AudioCodec: "aac", AudioStreams: 1},
			want:    "Los vídeos han salido sin subtítulos: este stream solo tiene una pista con la voz y el juego mezclados.",
			arrange: func(env streamTranscriptEnv) { env.transcriber.state = teamvoice.TranscriberInstalled }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newStreamTranscriptEnv(t)
			base := env.stream(tc.probe)
			tc.arrange(env)
			kind := "shorts"
			if tc.summary {
				kind = "summary"
			}
			env.json(http.MethodPatch, base+"/subtitles", map[string]bool{kind: true}, http.StatusOK, nil)
			req := stretch
			req.JobID, req.Summary = streamIDOf(base), tc.summary
			started := time.Now()
			got := env.h.StreamSubtitles(context.Background(), req)
			if got.Notice != tc.want || len(got.Lines) != 0 {
				t.Fatalf("render subtitles = %+v; want no frases and the notice %q", got, tc.want)
			}
			if strings.ContainsAny(got.Notice, "—–") {
				t.Fatalf("notice %q has a dash", got.Notice)
			}
			if waited := time.Since(started); waited > 3*time.Second {
				t.Fatalf("the render waited %s", waited)
			}
		})
	}
}

func (e streamTranscriptEnv) store(key string, value any) {
	e.t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.storage.Put(key, bytes.NewReader(raw)); err != nil {
		e.t.Fatal(err)
	}
}

func (e streamTranscriptEnv) stale(base string) streamSubtitlesStale {
	e.t.Helper()
	var got streamTranscript
	e.json(http.MethodGet, base+"/transcript", nil, http.StatusOK, &got)
	return got.Stale
}

func TestSubtitulosChangedSinceARenderMarkItsVideosStale(t *testing.T) {
	env := newStreamTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "clip-1", StartSeconds: 61, EndSeconds: 75}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	probe := micStream
	probe.DurationSeconds = 600
	job := &streamclips.Job{Status: streamclips.StatusRendered, SourcePath: "streams/source.mkv", SourceSHA256: uuid.NewString(), Probe: probe, EditPlan: planJSON}
	if err := env.streams.Create(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	base := "/api/stream-jobs/" + job.ID.String()
	state, err := streamclips.NewRenderState(job.ID, plan.Variant, streamclips.StatusRendered, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	stateKey, err := streamclips.RenderStateKey(job.ID, plan.Variant)
	if err != nil {
		t.Fatal(err)
	}
	result, err := streamclips.NewRenderResult(job.ID, plan.Variant, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	env.store(stateKey, state)
	env.store(state.ResultKey, result)
	plays := []vodplays.Play{{ID: "play-001", StartSeconds: 61, EndSeconds: 75}}
	env.store(streamclips.PlaysKey(job.ID), streamPlayScan{Status: playScanDone, Plays: plays})
	summary := streamSummary{Status: playScanDone, PlayIDs: []string{"play-001"}}
	env.store(streamclips.SummaryStateKey(job.ID), summary)

	if got := env.stale(base); got.Shorts || got.Summary {
		t.Fatalf("stale = %+v with subtítulos off; want the videos current", got)
	}
	env.json(http.MethodPatch, base+"/subtitles", map[string]bool{"shorts": true, "summary": true}, http.StatusOK, nil)
	env.json(http.MethodPost, base+"/transcript", streamSpans{Spans: []streamSpan{{StartSeconds: 61, EndSeconds: 75}}}, http.StatusAccepted, nil)
	ready := env.await(base, func(s streamTranscript) bool { return s.State == library.TranscriptReady })
	if !ready.Stale.Shorts || !ready.Stale.Summary {
		t.Fatalf("stale = %+v; want the videos rendered before their subtítulos marked to render again", ready.Stale)
	}

	track := streamclips.SubtitleTrack{Lines: ready.Lines}
	shorts, err := streamclips.NewSubtitleScripts(plan.SubtitleVideos(), track)
	if err != nil {
		t.Fatal(err)
	}
	result.SubtitlesFingerprint = shorts.Fingerprint
	env.store(state.ResultKey, result)
	resumen, err := streamclips.NewSubtitleScripts([]streamclips.SubtitleVideo{summarySubtitleVideo(plays)}, track)
	if err != nil {
		t.Fatal(err)
	}
	summary.SubtitlesFingerprint = resumen.Fingerprint
	env.store(streamclips.SummaryStateKey(job.ID), summary)
	if shorts.Fingerprint == "" || resumen.Fingerprint == "" {
		t.Fatalf("fingerprints = %q, %q; want both videos to draw the frase", shorts.Fingerprint, resumen.Fingerprint)
	}
	if got := env.stale(base); got.Shorts || got.Summary {
		t.Fatalf("stale = %+v after a render with these subtítulos; want the videos current", got)
	}

	hide := map[string]any{"video": "clip-1", "line_id": ready.Lines[0].ID, "hidden": true}
	env.json(http.MethodPatch, base+"/subtitles", hide, http.StatusOK, nil)
	if got := env.stale(base); !got.Shorts || got.Summary {
		t.Fatalf("stale = %+v after hiding a frase in the Short; want only the Shorts to render again", got)
	}
	env.json(http.MethodPatch, base+"/transcript/lines/"+ready.Lines[0].ID.String(), map[string]string{"text": "vaya tiro, tío"}, http.StatusOK, nil)
	if got := env.stale(base); !got.Summary {
		t.Fatalf("stale = %+v after correcting a frase; want the Resumen to render again", got)
	}
}
