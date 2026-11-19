package parser

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync/atomic"

	demoinfocs "github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs"
	"github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs/events"

	"github.com/rechedev9/cliphub/internal/demosafe"
)

// errPOVDemo is the "pov_demo: " failure for a demo a player recorded in their
// own client. CS2 plays it back as that player, never as an observer, so the
// capture script cannot lock any player's first-person view.
var errPOVDemo = errors.New("pov_demo: the demo was recorded by a player's client, not by GOTV; captures need the GOTV demo")

// parseToEnd is the single choke point every parse path (kills, smokes,
// roster, utility, tactical) runs a demo through; demosafe turns a parser panic into an error.
//
// Every failure but a cancellation or a read error is tagged "demo_incompatible: " so
// obs.ClassOf gives the job the demo-format FailureCode, panic or not.
func parseToEnd(p demoinfocs.Parser) error {
	return tagParseError(demosafe.ParseToEnd(p))
}

func tagParseError(parseErr error) error {
	switch {
	case parseErr == nil, errors.Is(parseErr, demoinfocs.ErrUnexpectedEndOfDemo):
		return nil
	case errors.Is(parseErr, demoinfocs.ErrCancelled), isReadError(parseErr):
		return parseErr
	default:
		return fmt.Errorf("demo_incompatible: %w", parseErr)
	}
}

// isReadError reports a failure of the file or the context under the demo, which a retry may not repeat.
func isReadError(err error) bool {
	_, ok := errors.AsType[*fs.PathError](err)
	return ok || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// parseCaptureDemo is parseToEnd for the paths that feed a capture (roster,
// kills, smokes, utility). It also refuses a client-recorded demo with errPOVDemo.
func parseCaptureDemo(p demoinfocs.Parser) error {
	_, err := parseCaptureDemoToEnd(p)
	return err
}

// parseCaptureDemoToEnd is parseCaptureDemo that also reports whether the
// demo stream stopped without its closing command.
func parseCaptureDemoToEnd(p demoinfocs.Parser) (endedEarly bool, err error) {
	var pov atomic.Bool
	p.RegisterEventHandler(func(e events.POVRecordingPlayerDetected) {
		// The library matches the header's client name against every user, so a
		// GOTV relay or a bot that shares it is not a recording player.
		if !e.PlayerInfo.IsHltv && !e.PlayerInfo.IsFakePlayer {
			pov.Store(true)
		}
	})
	parseErr := demosafe.ParseToEnd(p)
	err = tagParseError(parseErr)
	if pov.Load() && !errors.Is(err, demoinfocs.ErrCancelled) {
		return false, errPOVDemo
	}
	return errors.Is(parseErr, demoinfocs.ErrUnexpectedEndOfDemo), err
}
