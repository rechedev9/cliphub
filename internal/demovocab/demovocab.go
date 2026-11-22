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

type Sources struct {
	Document tacticalplan.Document
	Names    CalloutNames
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
	return v
}
