package recording

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/sharecode"
)

type CvarValue struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type FullDemoCaptureEvidence struct {
	SchemaVersion string         `json:"schema_version"`
	Before        []CvarValue    `json:"before"`
	Applied       []CvarValue    `json:"applied"`
	Restored      bool           `json:"restored"`
	FilesRestored bool           `json:"files_restored"`
	CertifiedEnds map[string]int `json:"certified_ends"`
}

// ReadFullDemoCaptureEvidence accepts only markers bound to this private run
// token. Console text in the source demo cannot authorize its own capture.
func ReadFullDemoCaptureEvidence(reader io.Reader, token string, plan RecordingPlan) (*FullDemoCaptureEvidence, error) {
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return nil, fmt.Errorf("full demo evidence token is required")
	}
	e := &FullDemoCaptureEvidence{SchemaVersion: "1.0", Before: []CvarValue{}, Applied: []CvarValue{}, CertifiedEnds: map[string]int{}}
	prefix := "ZV_FULL_DEMO:" + token + ":"
	bounded := &io.LimitedReader{R: reader, N: (256 << 20) + 1}
	scanner := bufio.NewScanner(bounded)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		var event struct {
			Kind     string      `json:"kind"`
			Values   []CvarValue `json:"values"`
			RoundID  string      `json:"round_id"`
			EndTick  int         `json:"end_tick"`
			Reason   string      `json:"reason"`
			Success  bool        `json:"success"`
			Failures []string    `json:"failures"`
		}
		dec := json.NewDecoder(strings.NewReader(strings.TrimPrefix(line, prefix)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&event); err != nil {
			return nil, fmt.Errorf("decode Full Demo runtime marker: %w", err)
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("trailing Full Demo runtime marker content")
		}
		switch event.Kind {
		case "settings_before":
			if len(e.Before) > 0 {
				return nil, fmt.Errorf("duplicate settings snapshot")
			}
			e.Before = event.Values
		case "settings_applied":
			if len(e.Applied) > 0 {
				return nil, fmt.Errorf("duplicate settings application")
			}
			e.Applied = event.Values
		case "settings_restored":
			e.Restored = event.Success && len(event.Failures) == 0
		case "certified_end":
			if _, exists := e.CertifiedEnds[event.RoundID]; exists {
				return nil, fmt.Errorf("duplicate certified capture interval")
			}
			e.CertifiedEnds[event.RoundID] = event.EndTick
		default:
			return nil, fmt.Errorf("unsupported Full Demo runtime evidence")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if bounded.N == 0 {
		return nil, fmt.Errorf("full demo console evidence exceeds 256 MiB")
	}
	return e, e.Validate(plan)
}

func (e *FullDemoCaptureEvidence) Validate(plan RecordingPlan) error {
	failed := func(detail string) error { return &recapplan.Error{Code: recapplan.ErrPOVContract, Detail: detail} }
	if e == nil || plan.FullDemo == nil || e.SchemaVersion != "1.0" || !e.Restored {
		return failed("Missing verified Full Demo settings restoration")
	}
	before, applied := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	for _, group := range []struct {
		values []CvarValue
		target map[string]json.RawMessage
	}{{e.Before, before}, {e.Applied, applied}} {
		if len(group.values) == 0 || len(group.values) > 512 {
			return failed("Invalid settings evidence size")
		}
		for _, v := range group.values {
			if v.Name == "" || len(v.Name) > 128 || len(v.Value) > 1024 || !json.Valid(v.Value) || bytes.Equal(v.Value, []byte("null")) {
				return failed("Invalid cvar evidence")
			}
			if _, duplicate := group.target[v.Name]; duplicate {
				return failed("Duplicate cvar evidence")
			}
			group.target[v.Name] = v.Value
		}
	}
	for name := range applied {
		if _, found := before[name]; !found {
			return failed("Applied cvar lacks original value: " + name)
		}
	}
	for _, name := range []string{"voice_modenable", "snd_voipvolume", "tv_listen_voice_indices", "tv_listen_voice_indices_h", "spec_show_xray", "spec_autodirector"} {
		if !slices.Contains([]string{"0", "false"}, string(applied[name])) {
			return failed("Required capture cvar was not disabled: " + name)
		}
	}
	wantCrosshair := "2"
	if plan.FullDemo.Options.Capture.Crosshair.Mode == "provided-code" {
		wantCrosshair = "0"
	}
	if string(applied["cl_show_observer_crosshair"]) != wantCrosshair {
		return failed("Crosshair source readback differs")
	}
	required := map[string]float64{"cl_drawhud": 1, "cl_draw_only_deathnotices": 0, "crosshair": 1, "cl_demo_predict": 0, "cl_trueview_show_status": 0}
	if plan.FullDemo.Options.Capture.HUDProfile == "native-clean-spectator" {
		for name, value := range map[string]float64{"cl_spec_show_bindings": 0, "cl_drawhud_specvote": 0, "cl_teamid_overhead_mode": 0, "cl_drawhud_force_teamid_overhead": -1, "hud_showtargetid": 0} {
			required[name] = value
		}
	}
	if plan.FullDemo.Options.Capture.Crosshair.Mode == "provided-code" {
		values, err := sharecode.CrosshairCvars(plan.FullDemo.Options.Capture.Crosshair.Code)
		if err != nil {
			return failed(err.Error())
		}
		for name, value := range values {
			required[name] = value
		}
	}
	for name, want := range required {
		var actual any
		if err := json.Unmarshal(applied[name], &actual); err != nil {
			return failed("Missing HUD/crosshair readback: " + name)
		}
		var number float64
		switch v := actual.(type) {
		case float64:
			number = v
		case bool:
			if v {
				number = 1
			}
		default:
			return failed("Invalid HUD/crosshair readback: " + name)
		}
		if math.IsNaN(number) || math.IsInf(number, 0) || math.Abs(number-want) > 0.00001 {
			return failed("HUD/crosshair readback differs: " + name)
		}
	}
	if len(e.CertifiedEnds) != len(plan.Segments) {
		return failed("Every captured interval needs a certified end")
	}
	for _, s := range plan.Segments {
		end, found := e.CertifiedEnds[s.ID]
		if !found || end <= s.TickStart || end > s.TickEnd || end < min(s.LiveEndTick+1, s.TickEnd) || (end < s.TickEnd && !plan.FullDemo.Options.Editorial.AllowSafeTailTrim) {
			return failed("Unapproved or interior capture gap: " + s.ID)
		}
	}
	return nil
}

// CertifiedPlan is a measurement view, not a replacement of the immutable
// launched plan or its input fingerprint.
func (r RecordingResult) CertifiedPlan() RecordingPlan {
	p := r.Plan
	if r.FullDemoEvidence == nil {
		return p
	}
	p.Segments = slices.Clone(p.Segments)
	for i := range p.Segments {
		if end, ok := r.FullDemoEvidence.CertifiedEnds[p.Segments[i].ID]; ok {
			p.Segments[i].TickEnd = end
		}
	}
	return p
}
