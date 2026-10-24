package parser

import (
	"errors"
	"fmt"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/demosafe"
)

// parseToEnd is the single choke point every parse path (kills, smokes,
// roster, utility) runs a demo through; demosafe turns a parser panic into an error.
//
// Every failure but a cancellation is tagged "demo_incompatible: " so obs.ClassOf
// gives the job the demo-format FailureCode, panic or not.
func parseToEnd(p demoinfocs.Parser) error {
	parseErr := demosafe.ParseToEnd(p)
	switch {
	case parseErr == nil, errors.Is(parseErr, demoinfocs.ErrUnexpectedEndOfDemo):
		return nil
	case errors.Is(parseErr, demoinfocs.ErrCancelled):
		return parseErr
	default:
		return fmt.Errorf("demo_incompatible: %w", parseErr)
	}
}
