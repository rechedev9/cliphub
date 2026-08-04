package streamclips

import (
	"encoding/json"
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
