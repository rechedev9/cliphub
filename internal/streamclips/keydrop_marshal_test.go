package streamclips

import (
	"encoding/json"
	"testing"
)

func TestKeyDropBannerMarshalIncludesCustomCode(t *testing.T) {
	start, end := 0.0, 4.0
	plan := DefaultEditPlan()
	plan.KeyDropBanner = KeyDropBannerPlan{
		Style: "classic",
		Code:  "CUSTOM123",
		StartSeconds: &start,
		EndSeconds: &end,
	}
	b, err := json.Marshal(NormalizeEditPlan(plan))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("json=%s", b)
	if !json.Valid(b) {
		t.Fatal("invalid json")
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	kd, ok := raw["keydrop_banner"].(map[string]any)
	if !ok {
		t.Fatalf("keydrop_banner missing: %s", b)
	}
	if kd["code"] != "CUSTOM123" {
		t.Fatalf("code field = %v, want CUSTOM123; full=%v", kd["code"], kd)
	}
	if kd["style"] != "classic" {
		t.Fatalf("style = %v", kd["style"])
	}
}
