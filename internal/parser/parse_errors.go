package parser

import (
	"errors"
	"fmt"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
)

// parseToEnd is the single choke point every parse path (kills, smokes,
// roster, utility) runs a demo through. demoinfocs recovers a plain
// truncated-stream panic into ErrUnexpectedEndOfDemo itself, but a demo that
// is corrupt or malformed in some other way (not just short) makes it
// re-panic out of ParseToEnd instead of returning an error. Left unrecovered,
// that panic crashes the entire orchestrator process, not just this one job
// — tolerable when the only demos in play are the desktop user's own, but
// not once a public portal lets strangers submit arbitrary files.
//
// Every parse failure other than a cancellation is tagged "demo_incompatible: "
// so obs.ClassOf gives the job the demo-format FailureCode and the web client
// can tell the user the demo itself is unreadable, whether demoinfocs panicked
// or returned an error (an unsupported file type, a message it cannot decode).
func parseToEnd(p demoinfocs.Parser) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("demo_incompatible: parser panicked: %v", r)
		}
	}()
	parseErr := p.ParseToEnd()
	switch {
	case parseErr == nil, errors.Is(parseErr, demoinfocs.ErrUnexpectedEndOfDemo):
		return nil
	case errors.Is(parseErr, demoinfocs.ErrCancelled):
		return parseErr
	default:
		return fmt.Errorf("demo_incompatible: %w", parseErr)
	}
}
