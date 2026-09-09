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
// not once a public portal lets strangers submit arbitrary files. The
// "demo_incompatible: " prefix matches the existing obs.ClassOf convention,
// so this failure gets the same FailureCode a demo-format problem already
// gets.
func parseToEnd(p demoinfocs.Parser) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("demo_incompatible: parser panicked: %v", r)
		}
	}()
	parseErr := p.ParseToEnd()
	if errors.Is(parseErr, demoinfocs.ErrUnexpectedEndOfDemo) {
		return nil
	}
	return parseErr
}
