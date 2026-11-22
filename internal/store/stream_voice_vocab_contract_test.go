package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestStreamVoiceLineKeepsWhatTheCorrectorWrote(t *testing.T) {
	for name, repo := range streamRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			id := newStreamForVoice(t, repo)
			heard, written := "moly on the left", "molly on the left"
			fixed := streamclips.VoiceLine{ID: uuid.New(), StartMS: 61_000, EndMS: 62_000, Text: written, OriginalText: heard, VocabText: written}
			old := streamclips.VoiceLine{ID: uuid.New(), StartMS: 63_000, EndMS: 64_000, Text: "vaya tiro"}
			spans := []library.VoiceSpan{{StartMS: 60_000, EndMS: 70_000}}
			set := func() {
				t.Helper()
				lines := []streamclips.VoiceLine{fixed, old}
				ready := streamclips.VoiceTranscript{State: library.TranscriptReady, Spans: spans}
				if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: id, Transcript: ready, Replaced: spans, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			call := func() streamclips.VoiceLine {
				t.Helper()
				got, err := repo.ListStreamVoiceLines(ctx, id)
				if err != nil || len(got) != 2 {
					t.Fatalf("lines = %+v, %v", got, err)
				}
				if got[1].VocabText != "" || got[1].OriginalText != "" || got[1].Text != "vaya tiro" {
					t.Fatalf("a frase the corrector left = %+v; want it unchanged", got[1])
				}
				return got[0]
			}
			patch := func(text string) streamclips.VoiceLine {
				t.Helper()
				line, err := repo.UpdateStreamVoiceLine(ctx, streamclips.VoiceLinePatch{JobID: id, ID: fixed.ID, Text: &text})
				if err != nil {
					t.Fatal(err)
				}
				return line
			}

			set()
			if got := call(); got.Text != written || got.VocabText != written || got.OriginalText != heard || got.Corrected {
				t.Fatalf("stored = %+v; want the corrector's text with the heard text beside it", got)
			}
			if got := patch(heard); got.Text != heard || !got.Corrected || got.OriginalText != "" || got.VocabText != written {
				t.Fatalf("undo of the corrector = %+v; want the heard text as a person's correction", got)
			}
			set()
			if got := call(); got.Text != heard || !got.Corrected || got.HeardText() != heard || got.VocabText != written {
				t.Fatalf("transcribed again after an undo = %+v; want the person's text kept", got)
			}
			if got := patch(written); got.Corrected || got.OriginalText != heard {
				t.Fatalf("back to the corrector's text = %+v; want it not corrected", got)
			}
			if got := patch("molotov on the left"); !got.Corrected || got.OriginalText != heard {
				t.Fatalf("a person's correction = %+v; want it corrected over the heard text", got)
			}
			set()
			if got := call(); got.Text != "molotov on the left" || !got.Corrected || got.OriginalText != heard || got.VocabText != written {
				t.Fatalf("transcribed again after a correction = %+v; want the person's text kept", got)
			}
		})
	}
}
