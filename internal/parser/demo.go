package parser

import (
	"context"
	"errors"
	"fmt"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// ErrTargetNotFound is returned when the requested target SteamID was never
// observed in the demo (neither as killer nor as victim). The CLI maps this
// to exit code 5.
var ErrTargetNotFound = errors.New("target steamid not found in demo")

type SegmentMode string

const (
	SegmentModeKills   SegmentMode = "kills"
	SegmentModeSmokes  SegmentMode = "smokes"
	SegmentModeUtility SegmentMode = "utility"
	SegmentModeRecap   SegmentMode = "recap"
)

// KnownSegmentModes is the public CLI allowlist, in catalog order.
func KnownSegmentModes() []SegmentMode {
	return []SegmentMode{SegmentModeKills, SegmentModeSmokes, SegmentModeUtility, SegmentModeRecap}
}

// ValidSegmentMode reports whether mode is a known public segment mode.
func ValidSegmentMode(mode SegmentMode) bool {
	switch mode {
	case SegmentModeKills, SegmentModeSmokes, SegmentModeUtility, SegmentModeRecap:
		return true
	default:
		return false
	}
}

type RunOptions struct {
	SegmentMode SegmentMode
}

// Run wires kill event handlers on p, drives the parser to completion, and
// returns the assembled kill plan. The passed PlanMeta supplies demo path
// and SHA256; map name, tickrate, and duration are filled in from the
// parser unless already provided.
func Run(p demoinfocs.Parser, target string, r rules.Rules, m PlanMeta) (killplan.Plan, error) {
	return RunWithOptions(p, target, r, m, RunOptions{SegmentMode: SegmentModeKills})
}

// RunWithContext drives the parser like RunWithOptions but aborts parsing when
// ctx is cancelled (e.g. an Asynq task deadline or a server shutdown), returning
// the context error instead of a partial plan. demoinfocs has no context-aware
// entry point, so a watcher goroutine calls p.Cancel() — which aborts
// ParseToEnd and drains its internal queues — when ctx is done.
//
// The caller still owns p and is responsible for Close(). The watcher is joined
// before this function returns, so Close() never races a Cancel() in flight.
func RunWithContext(ctx context.Context, p demoinfocs.Parser, target string, r rules.Rules, m PlanMeta, opts RunOptions) (killplan.Plan, error) {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	defer func() {
		close(stop)
		wg.Wait()
	}()

	plan, err := RunWithOptions(p, target, r, m, opts)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return killplan.Plan{}, fmt.Errorf("parse demo: %w", ctxErr)
	}
	return plan, err
}

func RunWithOptions(p demoinfocs.Parser, target string, r rules.Rules, m PlanMeta, opts RunOptions) (killplan.Plan, error) {
	watch := TrackFirstPacketTick(p)
	var plan killplan.Plan
	var err error
	switch opts.SegmentMode {
	case "", SegmentModeKills:
		plan, err = runKills(p, target, r, m)
	case SegmentModeSmokes:
		plan, err = runSmokes(p, target, r, m)
	case SegmentModeUtility:
		plan, err = runUtility(p, target, r, m)
	case SegmentModeRecap:
		plan, err = runRecap(p, target, r, m)
	default:
		return killplan.Plan{}, fmt.Errorf("unknown segment mode %q", opts.SegmentMode)
	}
	if err != nil {
		return plan, err
	}
	if tick, seen := watch.Snapshot(); seen {
		plan.Demo.FirstFullPacketTick = &tick
	}
	return plan, nil
}
