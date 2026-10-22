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
// Every parse failure other than a cancellation is tagged "demo_incompatible: "
// so obs.ClassOf gives the job the demo-format FailureCode and the web client
// can tell the user the demo itself is unreadable, whether demoinfocs panicked
// or returned an error (an unsupported file type, a message it cannot decode).
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
