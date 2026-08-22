package streamclips

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKeyDropBannerJSONRoundTripPreservesCode(t *testing.T) {
	raw := []byte(`{
		"schema_version":"1.1",
		"variant":"streamer-fullframe-nocam",
		"clips":[{"id":"c1","start_seconds":0,"end_seconds":10}],
		"keydrop_banner":{"style":"classic","code":"MIOTRO","slide_enabled":true,"start_seconds":0,"end_seconds":4}
	}`)
	plan, err := DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan = NormalizeEditPlan(plan)
	if plan.KeyDropBanner.Code != "MIOTRO" {
		t.Fatalf("code = %q, want MIOTRO", plan.KeyDropBanner.Code)
	}
	if plan.KeyDropBanner.Style != "classic" {
		t.Fatalf("style = %q", plan.KeyDropBanner.Style)
	}
	out, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := DecodeEditPlan(out)
	if err != nil {
		t.Fatal(err)
	}
	if plan2.KeyDropBanner.Code != "MIOTRO" {
		t.Fatalf("round-trip code = %q, want MIOTRO; json=%s", plan2.KeyDropBanner.Code, out)
	}
}

func TestEditPlanValidateAcceptsCatalogKeyDropStyles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		style   string
		wantErr string
	}{
		{style: "operator"},
		{style: "classic"},
		{style: "tigerr"},
		{style: "jcorko"},
		{style: ""},
		{style: "neon", wantErr: "unknown keydrop banner style"},
	}
	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			t.Parallel()
			plan := DefaultEditPlan()
			plan.Variant = VariantStreamerFullframeNoCam
			plan.Clips = []ClipRange{{ID: "c1", StartSeconds: 0, EndSeconds: 8}}
			plan.KeyDropBanner = KeyDropBannerPlan{Style: tt.style, Code: "TIGERR"}
			err := plan.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate(%q) = %v", tt.style, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate(%q) = %v, want substring %q", tt.style, err, tt.wantErr)
			}
		})
	}
}
