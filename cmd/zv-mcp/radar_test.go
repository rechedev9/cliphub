package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestResolveEditRadar(t *testing.T) {
	on, faint, half, full := true, 20, 50, 90
	edit, err := resolveEdit(shortChoices{Radar: &on, RadarPosition: "top-right", RadarSize: "large", RadarOpacity: &half})
	if err != nil {
		t.Fatal(err)
	}
	req := decodeEdit(t, edit)
	if !req.Radar || req.RadarPosition != "top-right" || req.RadarSize != "large" || req.RadarOpacity != 0.5 {
		t.Fatalf("radar not sent with its options: %+v", req)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("renderplan rejected the radar: %v", err)
	}
	if summary := describeShortRequest(tuiclient.GenerateRequest{Edit: edit}); !strings.Contains(summary, "radar top-right large al 50 %") {
		t.Errorf("summary misses the radar: %s", summary)
	}
	defaults, err := resolveEdit(shortChoices{Radar: &on, RadarPosition: "auto", RadarSize: "medium", RadarOpacity: &full})
	if err != nil || !defaults.Radar || defaults.RadarPosition != "" || defaults.RadarSize != "" || defaults.RadarOpacity != 0 {
		t.Fatalf("defaults = %+v, %v; want them unset like the server stores them", defaults, err)
	}
	for name, c := range map[string]shortChoices{
		"native HUD":      {NativeHUD: &on, Radar: &on},
		"unknown corner":  {Radar: &on, RadarPosition: "center"},
		"unknown size":    {Radar: &on, RadarSize: "huge"},
		"too faint":       {Radar: &on, RadarOpacity: &faint},
		"options without": {RadarSize: "large"},
	} {
		if _, err := resolveEdit(c); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
