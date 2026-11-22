package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func correctStreamFrases(t *testing.T, repos *store.Repositories, in streamFrases, written []string) uuid.UUID {
	t.Helper()
	id, lines := streamWithFrases(t, repos, in)
	voice, _ := repos.Streams.(store.StreamVoiceRepository)
	for i, text := range written {
		if _, err := voice.UpdateStreamVoiceLine(context.Background(), streamclips.VoiceLinePatch{JobID: id, ID: lines[i].ID, Text: &text}); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestTranscriptMeasureReplaysTheStreamVocabulary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	repos, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	measured := correctStreamFrases(t, repos, streamFrases{title: "Directo con moly", texts: []string{"moly here", "vesty now", "vesty go"}},
		[]string{"molly here", "vest now", "vest go"})
	correctStreamFrases(t, repos, streamFrases{title: "Otro directo", texts: []string{"vesty on him", "vesty again"}}, []string{"vest on him", "vest again"})
	if err := repos.Close(); err != nil {
		t.Fatal(err)
	}
	for _, row := range measureJSON(t, path).Transcripts {
		if row.ID != measured.String() {
			continue
		}
		if row.Result.VocabFixedWords != 3 || row.Result.VocabCorrectedWords != 0 || row.Result.VocabBrokenWords != 0 {
			t.Fatalf("stream row = %+v; want the lexicon and the rule learned from the other stream replayed", row.Result)
		}
		return
	}
	t.Fatal("the corrected stream is not measured")
}
