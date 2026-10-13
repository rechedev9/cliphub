package cloudbridge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func specWithWindows(tickrate int, windows ...captureWindow) jobSpec {
	var spec jobSpec
	spec.Capture.Tickrate = tickrate
	spec.Capture.Windows = windows
	return spec
}

func TestVerifyWindows(t *testing.T) {
	plan := func() *killplan.Plan {
		return &killplan.Plan{
			Demo: killplan.Demo{Tickrate: 64},
			Segments: []killplan.Segment{
				{ID: "seg-001", TickStart: 1000, TickEnd: 1640},
				{ID: "seg-002", TickStart: 9000, TickEnd: 9800},
				{ID: "seg-003", TickStart: 51234, TickEnd: 52010},
			},
		}
	}
	cases := []struct {
		name    string
		spec    jobSpec
		plan    *killplan.Plan
		wantErr string
	}{
		{
			name: "the selected windows are in the worker's plan",
			spec: specWithWindows(64, captureWindow{ID: "seg-003", TickStart: 51234, TickEnd: 52010}, captureWindow{ID: "seg-001", TickStart: 1000, TickEnd: 1640}),
			plan: plan(),
		},
		{
			name:    "a renumbered segment",
			spec:    specWithWindows(64, captureWindow{ID: "seg-004", TickStart: 51234, TickEnd: 52010}),
			plan:    plan(),
			wantErr: `window "seg-004" is not in the worker's plan`,
		},
		{
			// The id exists, but it names a different, longer window.
			name:    "an id that points at another window",
			spec:    specWithWindows(64, captureWindow{ID: "seg-002", TickStart: 51234, TickEnd: 52010}),
			plan:    plan(),
			wantErr: `window "seg-002" covers ticks 51234 to 52010 in the spec and 9000 to 9800`,
		},
		{
			name:    "a shifted start tick",
			spec:    specWithWindows(64, captureWindow{ID: "seg-003", TickStart: 51200, TickEnd: 52010}),
			plan:    plan(),
			wantErr: `window "seg-003" covers ticks 51200 to 52010`,
		},
		{
			name:    "a shifted end tick",
			spec:    specWithWindows(64, captureWindow{ID: "seg-003", TickStart: 51234, TickEnd: 52011}),
			plan:    plan(),
			wantErr: `window "seg-003" covers ticks 51234 to 52011`,
		},
		{
			// The portal computed the cost with the spec's tickrate.
			name:    "another tickrate",
			spec:    specWithWindows(128, captureWindow{ID: "seg-003", TickStart: 51234, TickEnd: 52010}),
			plan:    plan(),
			wantErr: "tickrate is 128 in the spec and 64 in the demo",
		},
		{
			name:    "no plan at all",
			spec:    specWithWindows(64, captureWindow{ID: "seg-003", TickStart: 51234, TickEnd: 52010}),
			wantErr: "no kill plan",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyWindows(tc.spec, tc.plan)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("verifyWindows error = %v, want nil", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("verifyWindows error = %v, want one mentioning %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseJobSpecRefusesWhatWouldRecordMoreThanTheWindows(t *testing.T) {
	replace := func(old, replacement string) json.RawMessage {
		spec := string(testSpec())
		if !strings.Contains(spec, old) {
			t.Fatalf("test spec does not contain %q", old)
		}
		return json.RawMessage(strings.Replace(spec, old, replacement, 1))
	}
	cases := []struct {
		name    string
		kind    string
		spec    json.RawMessage
		wantErr string
	}{
		{name: "a valid short", kind: kindShort, spec: testSpec()},
		{name: "rules left out", kind: kindShort, spec: replace(`"rules":{"weapons":["awp"],"min_kills_in_window":2,"window_seconds":10,"pre_roll_seconds":3,"post_roll_seconds":4,"min_round":1}`, `"rules":{}`)},
		{name: "a kind this worker does not produce", kind: "full_demo", spec: testSpec(), wantErr: `kind "full_demo"`},
		{name: "another spec version", kind: kindShort, spec: replace(`"version":1`, `"version":2`), wantErr: "unsupported spec version 2"},
		{name: "a target that is not a SteamID64", kind: kindShort, spec: replace(testTarget, "1234"), wantErr: "targetSteamId"},
		{name: "no windows", kind: kindShort, spec: replace(`"windows":[{"id":"seg-003","tickStart":51234,"tickEnd":52010}]`, `"windows":[]`), wantErr: "no capture windows"},
		{name: "rules the parser rejects", kind: kindShort, spec: replace(`"min_kills_in_window":2`, `"min_kills_in_window":0`), wantErr: "min_kills_in_window"},
		{name: "an empty selection, which means every segment", kind: kindShort, spec: replace(`"segment_ids":["seg-003"]`, `"segment_ids":[]`), wantErr: "segment_ids"},
		{name: "a selection beyond the windows", kind: kindShort, spec: replace(`"segment_ids":["seg-003"]`, `"segment_ids":["seg-003","seg-001"]`), wantErr: "segment_ids"},
		{name: "a recap edit, which records the whole match", kind: kindShort, spec: replace(`"intro":true`, `"intro":true,"match_recap":true`), wantErr: "Full Demo"},
		{name: "a Full Demo document", kind: kindShort, spec: replace(`"intro":true`, `"intro":true,"full_demo":{"document":{}}`), wantErr: "Full Demo"},
		{name: "not JSON", kind: kindShort, spec: json.RawMessage(`{"version":`), wantErr: "not valid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, jobRules, err := parseJobSpec(tc.kind, tc.spec)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseJobSpec error = %v, want one mentioning %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJobSpec error = %v, want nil", err)
			}
			if spec.TargetSteamID != testTarget || len(spec.Capture.Windows) != 1 || string(spec.Generate) != testGenerate {
				t.Fatalf("spec = %+v", spec)
			}
			if err := jobRules.Validate(); err != nil {
				t.Fatalf("parsed rules do not validate: %v", err)
			}
		})
	}
}
