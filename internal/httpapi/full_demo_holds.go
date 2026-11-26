package httpapi

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/rechedev9/cliphub/internal/demosafe"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

type killHoldsBackfill struct {
	mu   sync.Mutex
	read func(context.Context, storage.Storage, job.Job) (*recapplan.HoldEvidence, error)
}

func (h *Handlers) ensureKillHolds(ctx context.Context, j job.Job, facts recapplan.Facts) (recapplan.Facts, error) {
	h.killHolds.mu.Lock()
	defer h.killHolds.mu.Unlock()
	stored, found, err := recapplan.LoadFacts(h.storage, j.ID)
	if err != nil {
		return facts, err
	}
	if found && stored.Holds != nil {
		facts.Holds = stored.Holds
		return facts, nil
	}
	read := h.killHolds.read
	if read == nil {
		read = parseKillHolds
	}
	holds, err := read(ctx, h.storage, j)
	if err != nil {
		if ctx.Err() != nil {
			return facts, ctx.Err()
		}
		log.Printf("httpapi: read the enemy movement of job %s for the second camera: %v", j.ID, err)
		return facts, nil
	}
	if holds == nil {
		return facts, nil
	}
	facts.Holds = holds
	if err := recapplan.StoreFacts(h.storage, j.ID, facts); err != nil {
		return facts, fmt.Errorf("store the hold evidence: %w", err)
	}
	return facts, nil
}

func parseKillHolds(ctx context.Context, store storage.Storage, j job.Job) (*recapplan.HoldEvidence, error) {
	demo, err := store.Open(j.DemoPath)
	if err != nil {
		return nil, fmt.Errorf("open demo: %w", err)
	}
	defer demo.Close()
	p, err := demosafe.NewParser(demo)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	dual, err := parser.RunKillsAndRecapWithContext(ctx, p, j.TargetSteamID, j.Rules, parser.PlanMeta{DemoPath: j.DemoPath, SHA256: j.DemoSHA256})
	if err != nil {
		return nil, err
	}
	return dual.Facts.Holds, nil
}
