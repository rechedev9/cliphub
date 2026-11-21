package store

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type streamVoiceRepository interface {
	StreamJobRepository
	StreamVoiceRepository
}

func streamRepositoriesUnderTest(t *testing.T) map[string]streamVoiceRepository {
	t.Helper()
	return map[string]streamVoiceRepository{"sqlite": newTestSQLiteStreamRepo(t), "memory": NewMemoryStreamJobRepository()}
}

func newStreamForVoice(t *testing.T, repo streamVoiceRepository) uuid.UUID {
	t.Helper()
	job := &streamclips.Job{Status: streamclips.StatusReady, SourcePath: "streams/source.mkv", SourceSHA256: uuid.NewString()}
	if err := repo.Create(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	return job.ID
}

func TestStreamVoiceRepositoryContract(t *testing.T) {
	for name, repo := range streamRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			id, other := newStreamForVoice(t, repo), newStreamForVoice(t, repo)
			if _, err := repo.GetStreamVoice(ctx, uuid.New()); !errors.Is(err, streamclips.ErrNotFound) {
				t.Fatalf("GetStreamVoice(unknown stream) = %v, want ErrNotFound", err)
			}
			none, err := repo.GetStreamVoice(ctx, id)
			if err != nil || none.State != library.TranscriptNone || none.JobID != id || len(none.Spans) != 0 {
				t.Fatalf("GetStreamVoice(none) = %+v, %v; want the none state", none, err)
			}
			queued := streamclips.VoiceTranscript{State: library.TranscriptQueued, Pending: []library.VoiceSpan{{StartMS: 60_000, EndMS: 80_000}}}
			if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: uuid.New(), Transcript: queued}); !errors.Is(err, streamclips.ErrNotFound) {
				t.Fatalf("SetStreamVoice(unknown stream) = %v, want ErrNotFound", err)
			}
			if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: other, Transcript: queued}); err != nil {
				t.Fatal(err)
			}
			words := []library.DemoVoiceWord{{Text: "vaya", StartMS: 61_000, EndMS: 61_400}, {Text: "tiro", StartMS: 61_500, EndMS: 62_000}}
			shot := streamclips.VoiceLine{ID: uuid.New(), StartMS: 61_000, EndMS: 62_000, Text: "vaya tiro", Words: words}
			laugh := streamclips.VoiceLine{ID: uuid.New(), StartMS: 10_000, EndMS: 11_000, Text: "ja ja"}
			lines := []streamclips.VoiceLine{shot, laugh}
			spans := []library.VoiceSpan{{StartMS: 0, EndMS: 20_000}, {StartMS: 60_000, EndMS: 80_000}}
			ready := streamclips.VoiceTranscript{State: library.TranscriptReady, Model: "parakeet", Spans: spans}
			if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: id, Transcript: ready, Replaced: spans, Lines: &lines}); err != nil {
				t.Fatal(err)
			}
			words[0].Text = "changed by the caller"
			got, err := repo.ListStreamVoiceLines(ctx, id)
			if err != nil || len(got) != 2 || got[0].ID != laugh.ID || got[1].JobID != id || len(got[0].Words) != 0 ||
				!reflect.DeepEqual(got[1].Words, []library.DemoVoiceWord{{Text: "vaya", StartMS: 61_000, EndMS: 61_400}, {Text: "tiro", StartMS: 61_500, EndMS: 62_000}}) {
				t.Fatalf("lines = %+v, %v; want time order with the stored words", got, err)
			}
			state, err := repo.GetStreamVoice(ctx, id)
			if err != nil || state.State != library.TranscriptReady || state.Model != "parakeet" || !reflect.DeepEqual(state.Spans, spans) || len(state.Pending) != 0 {
				t.Fatalf("GetStreamVoice = %+v, %v", state, err)
			}
			pending, err := repo.ListStreamVoices(ctx, []string{library.TranscriptQueued, library.TranscriptRunning})
			if err != nil || len(pending) != 1 || pending[0].JobID != other || !reflect.DeepEqual(pending[0].Pending, queued.Pending) {
				t.Fatalf("ListStreamVoices = %+v, %v; want the queued stream with its waiting stretches", pending, err)
			}

			fixed := "vaya tiro, tío"
			line, err := repo.UpdateStreamVoiceLine(ctx, streamclips.VoiceLinePatch{JobID: id, ID: shot.ID, Text: &fixed})
			if err != nil || line.Text != fixed || !line.Corrected || line.OriginalText != "vaya tiro" || len(line.Words) != 3 || line.Words[2].EndMS != 62_000 {
				t.Fatalf("correction = %+v, %v; want the original kept and the words spread again", line, err)
			}
			if _, err := repo.UpdateStreamVoiceLine(ctx, streamclips.VoiceLinePatch{JobID: other, ID: shot.ID, Text: &fixed}); !errors.Is(err, streamclips.ErrVoiceLineNotFound) {
				t.Fatalf("a correction through another stream = %v, want ErrVoiceLineNotFound", err)
			}
			yes := true
			if line, err = repo.UpdateStreamVoiceLine(ctx, streamclips.VoiceLinePatch{JobID: id, ID: laugh.ID, Discarded: &yes}); err != nil || !line.Discarded || line.Text != "ja ja" {
				t.Fatalf("discard = %+v, %v", line, err)
			}

			again := []streamclips.VoiceLine{{ID: shot.ID, StartMS: 61_000, EndMS: 62_000, Text: "vaya tiro"}, {ID: uuid.New(), StartMS: 70_000, EndMS: 71_000, Text: "otra"}}
			if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: id, Transcript: ready, Replaced: spans[1:], Lines: &again}); err != nil {
				t.Fatal(err)
			}
			got, _ = repo.ListStreamVoiceLines(ctx, id)
			if len(got) != 3 || got[0].ID != laugh.ID || !got[0].Discarded || got[1].Text != fixed || got[1].OriginalText != "vaya tiro" || got[2].Text != "otra" {
				t.Fatalf("lines after transcribing one stretch again = %+v; want the other stretch untouched and the correction kept", got)
			}
			if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: id, Transcript: ready}); err != nil {
				t.Fatal(err)
			}
			if got, _ = repo.ListStreamVoiceLines(ctx, id); len(got) != 3 {
				t.Fatalf("lines after a state update = %+v, want them kept", got)
			}

			if err := repo.Delete(ctx, id); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.GetStreamVoice(ctx, id); !errors.Is(err, streamclips.ErrNotFound) {
				t.Fatalf("transcript after the stream went = %v, want ErrNotFound", err)
			}
			if left, _ := repo.ListStreamVoiceLines(ctx, id); len(left) != 0 {
				t.Fatalf("lines after the stream went = %+v", left)
			}
		})
	}
}

func TestStreamSubtitlesRepositoryContract(t *testing.T) {
	for name, repo := range streamRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			id := newStreamForVoice(t, repo)
			if _, err := repo.GetStreamSubtitles(ctx, uuid.New()); !errors.Is(err, streamclips.ErrNotFound) {
				t.Fatalf("GetStreamSubtitles(unknown stream) = %v, want ErrNotFound", err)
			}
			off, err := repo.GetStreamSubtitles(ctx, id)
			if err != nil || off.Shorts || off.Summary || len(off.Hidden) != 0 || off.JobID != id {
				t.Fatalf("GetStreamSubtitles(none) = %+v, %v; want everything off", off, err)
			}
			frase := uuid.New()
			hidden := map[string][]uuid.UUID{"clip-1": {frase}}
			if err := repo.SetStreamSubtitles(ctx, streamclips.Subtitles{JobID: uuid.New(), Shorts: true}); !errors.Is(err, streamclips.ErrNotFound) {
				t.Fatalf("SetStreamSubtitles(unknown stream) = %v, want ErrNotFound", err)
			}
			if err := repo.SetStreamSubtitles(ctx, streamclips.Subtitles{JobID: id, Shorts: true, Hidden: hidden}); err != nil {
				t.Fatal(err)
			}
			hidden["clip-1"][0] = uuid.New()
			got, err := repo.GetStreamSubtitles(ctx, id)
			if err != nil || !got.Shorts || got.Summary || !reflect.DeepEqual(got.Hidden["clip-1"], []uuid.UUID{frase}) || len(got.Hidden[streamclips.SummaryVideo]) != 0 {
				t.Fatalf("GetStreamSubtitles = %+v, %v; want Shorts on and the frase hidden in its video only", got, err)
			}
			if err := repo.SetStreamSubtitles(ctx, streamclips.Subtitles{JobID: id, Summary: true}); err != nil {
				t.Fatal(err)
			}
			if got, _ = repo.GetStreamSubtitles(ctx, id); got.Shorts || !got.Summary || len(got.Hidden) != 0 {
				t.Fatalf("GetStreamSubtitles after a replace = %+v", got)
			}
			if err := repo.Delete(ctx, id); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.GetStreamSubtitles(ctx, id); !errors.Is(err, streamclips.ErrNotFound) {
				t.Fatalf("subtitles after the stream went = %v, want ErrNotFound", err)
			}
		})
	}
}
