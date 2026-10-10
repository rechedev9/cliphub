package cloudbridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

const kindShort = "short"

// captureWindow is one tick range the user chose and the portal charged for.
type captureWindow struct {
	ID        string `json:"id"`
	TickStart int    `json:"tickStart"`
	TickEnd   int    `json:"tickEnd"`
}

// jobSpec is the job description the portal stores and sends unchanged.
// Generate is passed through to the local pipeline byte for byte.
type jobSpec struct {
	Version       int             `json:"version"`
	TargetSteamID string          `json:"targetSteamId"`
	Rules         json.RawMessage `json:"rules"`
	Capture       struct {
		Tickrate int             `json:"tickrate"`
		Windows  []captureWindow `json:"windows"`
	} `json:"capture"`
	Generate json.RawMessage `json:"generate"`
}

var targetSteamIDPattern = regexp.MustCompile(`^[0-9]{17}$`)

// parseJobSpec decodes a claimed spec and rejects what this worker must not
// run. The portal validates the same envelope; the worker does not rely on
// it, because the spec decides how long CS2 records on this machine.
func parseJobSpec(kind string, raw json.RawMessage) (jobSpec, rules.Rules, error) {
	var spec jobSpec
	if kind != kindShort {
		return spec, rules.Rules{}, fmt.Errorf("this worker does not produce kind %q", kind)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, rules.Rules{}, fmt.Errorf("spec is not valid JSON: %w", err)
	}
	if spec.Version != 1 {
		return spec, rules.Rules{}, fmt.Errorf("unsupported spec version %d", spec.Version)
	}
	if !targetSteamIDPattern.MatchString(spec.TargetSteamID) {
		return spec, rules.Rules{}, errors.New("targetSteamId must be a 17 digit SteamID64")
	}
	if len(spec.Capture.Windows) == 0 {
		return spec, rules.Rules{}, errors.New("spec has no capture windows")
	}
	jobRules := rules.Default()
	if len(spec.Rules) > 0 && string(spec.Rules) != "null" {
		if err := json.Unmarshal(spec.Rules, &jobRules); err != nil {
			return spec, rules.Rules{}, fmt.Errorf("rules are not valid: %w", err)
		}
	}
	if err := jobRules.Validate(); err != nil {
		return spec, rules.Rules{}, err
	}

	// An empty segment list or a recap edit would make the local pipeline
	// record more than the verified windows, so both are refused here.
	var generate struct {
		SegmentIDs []string `json:"segment_ids"`
		Edit       *struct {
			FullDemo   json.RawMessage `json:"full_demo"`
			MatchRecap bool            `json:"match_recap"`
		} `json:"edit"`
	}
	if err := json.Unmarshal(spec.Generate, &generate); err != nil {
		return spec, rules.Rules{}, fmt.Errorf("generate is not a JSON object: %w", err)
	}
	windowIDs := make([]string, len(spec.Capture.Windows))
	for i, window := range spec.Capture.Windows {
		windowIDs[i] = window.ID
	}
	if !slices.Equal(generate.SegmentIDs, windowIDs) {
		return spec, rules.Rules{}, errors.New("generate.segment_ids must list exactly the capture windows, in order")
	}
	if generate.Edit != nil && (generate.Edit.MatchRecap || (len(generate.Edit.FullDemo) > 0 && string(generate.Edit.FullDemo) != "null")) {
		return spec, rules.Rules{}, errors.New("a short cannot carry a Full Demo edit")
	}
	return spec, jobRules, nil
}

// verifyWindows checks every window of the spec against the kill plan this
// worker parsed from the same demo. The user's plan is not trusted: a window
// the worker's own plan does not contain, at the same ticks, is a mismatch.
func verifyWindows(spec jobSpec, plan *killplan.Plan) error {
	if plan == nil {
		return errors.New("the worker has no kill plan for this demo")
	}
	if spec.Capture.Tickrate != plan.Demo.Tickrate {
		return fmt.Errorf("tickrate is %d in the spec and %d in the demo", spec.Capture.Tickrate, plan.Demo.Tickrate)
	}
	segments := make(map[string]killplan.Segment, len(plan.Segments))
	for _, segment := range plan.Segments {
		segments[segment.ID] = segment
	}
	for _, window := range spec.Capture.Windows {
		segment, found := segments[window.ID]
		if !found {
			return fmt.Errorf("window %q is not in the worker's plan", window.ID)
		}
		if window.TickStart != segment.TickStart || window.TickEnd != segment.TickEnd {
			return fmt.Errorf(
				"window %q covers ticks %d to %d in the spec and %d to %d in the worker's plan",
				window.ID, window.TickStart, window.TickEnd, segment.TickStart, segment.TickEnd,
			)
		}
	}
	return nil
}
