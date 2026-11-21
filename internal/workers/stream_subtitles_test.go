package workers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
)

type fixedStreamSubtitles struct {
	track streamclips.SubtitleTrack
	asked []streamclips.SubtitleRequest
}

func (f *fixedStreamSubtitles) StreamSubtitles(_ context.Context, req streamclips.SubtitleRequest) streamclips.SubtitleTrack {
	f.asked = append(f.asked, req)
	return f.track
}

type renderedStream struct {
	graphs []string
	state  streamclips.RenderState
	result streamclips.RenderResult
}

func renderStreamWithSubtitles(t *testing.T, source *fixedStreamSubtitles) renderedStream {
	t.Helper()
	store := newFakeStorage()
	id, plan := newReadyStreamJob(t, store)
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 60, EndSeconds: 70, Title: "one"}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id),
		Probe: streamclips.SourceProbe{DurationSeconds: 600}, EditPlan: planJSON})
	var out renderedStream
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if at := slices.Index(args, "-filter_complex"); at >= 0 {
			out.graphs = append(out.graphs, args[at+1])
		}
		target := args[len(args)-1]
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(target, []byte("video"), 0o600)
	}}
	w := NewStreamRenderWorker(repo, store, StreamRenderWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: "ffmpeg"})
	w.runner = runner
	w.UseSubtitles(source)
	task, err := tasks.NewRenderStreamClipTask(id, plan.Variant)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.HandleRenderStreamClip(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderStreamClip error = %v", err)
	}
	stateKey, err := streamclips.RenderStateKey(id, plan.Variant)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(store.files[stateKey], &out.state); err != nil {
		t.Fatalf("decode render state: %v", err)
	}
	if err := json.Unmarshal(store.files[out.state.ResultKey], &out.result); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	if len(source.asked) != 1 || source.asked[0].JobID != id || source.asked[0].Summary || !slices.Equal(source.asked[0].Stretches, []library.VoiceSpan{{StartMS: 60_000, EndMS: 70_000}}) {
		t.Fatalf("the render asked for %+v; want the frases of its momento", source.asked)
	}
	return out
}

func TestStreamRenderDrawsTheSubtitlesOfItsMomentos(t *testing.T) {
	frase := streamclips.VoiceLine{ID: uuid.New(), StartMS: 61_000, EndMS: 62_000, Text: "vaya tiro"}
	got := renderStreamWithSubtitles(t, &fixedStreamSubtitles{track: streamclips.SubtitleTrack{Lines: []streamclips.VoiceLine{frase}}})
	if len(got.graphs) != 1 || !strings.Contains(got.graphs[0], "ass=filename=") || !strings.Contains(got.graphs[0], "clip-001.ass") {
		t.Fatalf("clip filter graphs = %q; want the subtitle script of the momento drawn", got.graphs)
	}
	if got.result.SubtitlesFingerprint == "" || strings.Contains(strings.Join(got.state.Warnings, " "), "subtítulos") {
		t.Fatalf("fingerprint = %q, warnings = %q; want the render to record what it drew", got.result.SubtitlesFingerprint, got.state.Warnings)
	}
}

func TestStreamRenderWithoutSubtitlesSaysSo(t *testing.T) {
	notice := "Los vídeos han salido sin subtítulos: la transcripción falló en este PC."
	got := renderStreamWithSubtitles(t, &fixedStreamSubtitles{track: streamclips.SubtitleTrack{Notice: notice}})
	if len(got.graphs) != 1 || strings.Contains(got.graphs[0], "ass=") {
		t.Fatalf("clip filter graphs = %q; want the momento rendered without subtitles", got.graphs)
	}
	if !slices.Contains(got.state.Warnings, notice) || !slices.Contains(got.result.Warnings, notice) || got.result.SubtitlesFingerprint != "" {
		t.Fatalf("state warnings = %q, result = %+v; want the notice and no fingerprint", got.state.Warnings, got.result)
	}
}
