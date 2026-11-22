package demovocab

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/vocab"
)

type CalloutNames interface {
	ListCalloutNames(ctx context.Context, mapName string) ([]library.CalloutName, error)
}

type DemoFrases interface {
	ListDemoVoiceLines(ctx context.Context, filter library.DemoVoiceLineFilter) ([]library.DemoVoiceLine, error)
}

type Sources struct {
	Document tacticalplan.Document
	Names    CalloutNames
	Frases   DemoFrases
}

func DemoEdits(ctx context.Context, frases DemoFrases) []vocab.Edit {
	lines, err := frases.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{All: true})
	if err != nil {
		log.Printf("demovocab: read the library frases for the learned corrections: %v", err)
		return nil
	}
	edits := []vocab.Edit{}
	for _, l := range lines {
		if l.Corrected {
			edits = append(edits, vocab.Edit{Heard: l.HeardText(), Machine: l.VocabText, Written: l.Text})
		}
	}
	return edits
}

type StreamFrases interface {
	ListStreamVoices(ctx context.Context, states []string) ([]streamclips.VoiceTranscript, error)
	ListStreamVoiceLines(ctx context.Context, jobID uuid.UUID) ([]streamclips.VoiceLine, error)
}

type StreamSources struct {
	Demos   DemoFrases
	Streams StreamFrases
}

func StreamEdits(ctx context.Context, frases StreamFrases) []vocab.Edit {
	transcripts, err := frases.ListStreamVoices(ctx, []string{library.TranscriptQueued, library.TranscriptRunning, library.TranscriptReady, library.TranscriptFailed})
	if err != nil {
		log.Printf("demovocab: read the stream transcripts for the learned corrections: %v", err)
		return nil
	}
	edits := []vocab.Edit{}
	for _, t := range transcripts {
		lines, err := frases.ListStreamVoiceLines(ctx, t.JobID)
		if err != nil {
			log.Printf("demovocab: read the frases of stream %s for the learned corrections: %v", t.JobID, err)
			continue
		}
		for _, l := range lines {
			if l.Corrected {
				edits = append(edits, vocab.Edit{Heard: l.HeardText(), Machine: l.VocabText, Written: l.Text})
			}
		}
	}
	return edits
}

func BuildStream(ctx context.Context, in StreamSources) vocab.Vocabulary {
	var edits []vocab.Edit
	if in.Demos != nil {
		edits = DemoEdits(ctx, in.Demos)
	}
	if in.Streams != nil {
		edits = append(edits, StreamEdits(ctx, in.Streams)...)
	}
	return vocab.Vocabulary{Terms: vocab.Lexicon(""), Learned: vocab.Learn(edits)}
}

func Build(ctx context.Context, in Sources) vocab.Vocabulary {
	doc := in.Document
	v := vocab.Vocabulary{}
	for _, c := range doc.Geometry.Callouts {
		if c.Name != "" {
			v.Terms = append(v.Terms, vocab.Term{Text: placename.Label(c.Name)})
		}
	}
	if mapName := doc.Demo.Map; mapName != "" {
		names, err := in.Names.ListCalloutNames(ctx, mapName)
		if err != nil {
			log.Printf("demovocab: read the callout names of %s for the corrector: %v", mapName, err)
		}
		for _, n := range names {
			v.Terms = append(v.Terms, vocab.Term{Text: n.Name})
		}
	}
	for _, p := range doc.Players {
		v.Terms = append(v.Terms, vocab.Term{Text: p.Name, KeepCase: true})
	}
	v.Terms = append(v.Terms, vocab.Lexicon(doc.Demo.Map)...)
	if in.Frases != nil {
		v.Learned = vocab.Learn(DemoEdits(ctx, in.Frases))
	}
	return v
}
