package demovocab

import (
	"context"
	"log"
	"slices"

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

type Glossary struct {
	Players  []string `json:"players"`
	Callouts []string `json:"callouts"`
	Lexicon  []string `json:"lexicon"`
}

func (g Glossary) terms() []vocab.Term {
	out := make([]vocab.Term, 0, len(g.Callouts)+len(g.Players)+len(g.Lexicon))
	for _, c := range g.Callouts {
		out = append(out, vocab.Term{Text: c})
	}
	for _, p := range g.Players {
		out = append(out, vocab.Term{Text: p, KeepCase: true})
	}
	for _, l := range g.Lexicon {
		out = append(out, vocab.Term{Text: l})
	}
	return out
}

func lexicon(mapName string) []string {
	terms := vocab.Lexicon(mapName)
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		out = appendNew(out, t.Text)
	}
	return out
}

func appendNew(list []string, word string) []string {
	if word == "" || slices.Contains(list, word) {
		return list
	}
	return append(list, word)
}

func StreamGlossary() Glossary {
	return Glossary{Players: []string{}, Callouts: []string{}, Lexicon: lexicon("")}
}

func DemoGlossary(ctx context.Context, in Sources) Glossary {
	doc := in.Document
	g := Glossary{Players: []string{}, Callouts: []string{}, Lexicon: lexicon(doc.Demo.Map)}
	for _, c := range doc.Geometry.Callouts {
		if c.Name != "" {
			g.Callouts = appendNew(g.Callouts, placename.Label(c.Name))
		}
	}
	if mapName := doc.Demo.Map; mapName != "" {
		names, err := in.Names.ListCalloutNames(ctx, mapName)
		if err != nil {
			log.Printf("demovocab: read the callout names of %s for the corrector: %v", mapName, err)
		}
		for _, n := range names {
			g.Callouts = appendNew(g.Callouts, n.Name)
		}
	}
	for _, p := range doc.Players {
		g.Players = appendNew(g.Players, p.Name)
	}
	return g
}

func BuildStream(ctx context.Context, in StreamSources) vocab.Vocabulary {
	var learnFrom []vocab.Edit
	if in.Demos != nil {
		learnFrom = edits(in.Demos.ListDemoVoiceCorrections(ctx))
	}
	if in.Streams != nil {
		learnFrom = append(learnFrom, edits(in.Streams.ListStreamVoiceCorrections(ctx))...)
	}
	terms := StreamGlossary().terms()
	return vocab.Vocabulary{Terms: terms, Learned: vocab.Learn(learnFrom, terms)}
}

func Build(ctx context.Context, in Sources) vocab.Vocabulary {
	v := vocab.Vocabulary{Terms: DemoGlossary(ctx, in).terms()}
	if in.Corrections != nil {
		v.Learned = vocab.Learn(edits(in.Corrections.ListDemoVoiceCorrections(ctx)), v.Terms)
	}
	return v
}
