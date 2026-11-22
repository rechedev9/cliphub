package demovocab

import (
	"context"
	"log"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/vocab"
)

type CalloutNames interface {
	ListCalloutNames(ctx context.Context, mapName string) ([]library.CalloutName, error)
}

type DemoCorrections interface {
	ListDemoVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error)
}

type StreamCorrections interface {
	ListStreamVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error)
}

type Sources struct {
	Document    tacticalplan.Document
	Names       CalloutNames
	Corrections DemoCorrections
}

type StreamSources struct {
	Demos   DemoCorrections
	Streams StreamCorrections
}

func edits(corrections []library.VoiceCorrection, err error) []vocab.Edit {
	if err != nil {
		log.Printf("demovocab: read the corrected frases for the learned corrections: %v", err)
		return nil
	}
	out := make([]vocab.Edit, 0, len(corrections))
	for _, c := range corrections {
		out = append(out, vocab.Edit{Heard: c.Heard, Machine: c.Vocab, Written: c.Text})
	}
	return out
}

func BuildStream(ctx context.Context, in StreamSources) vocab.Vocabulary {
	var learnFrom []vocab.Edit
	if in.Demos != nil {
		learnFrom = edits(in.Demos.ListDemoVoiceCorrections(ctx))
	}
	if in.Streams != nil {
		learnFrom = append(learnFrom, edits(in.Streams.ListStreamVoiceCorrections(ctx))...)
	}
	terms := vocab.Lexicon("")
	return vocab.Vocabulary{Terms: terms, Learned: vocab.Learn(learnFrom, terms)}
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
	if in.Corrections != nil {
		v.Learned = vocab.Learn(edits(in.Corrections.ListDemoVoiceCorrections(ctx)), v.Terms)
	}
	return v
}
