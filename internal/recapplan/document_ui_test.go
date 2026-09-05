package recapplan

import (
	"encoding/json"
	"os"
	"testing"
)

// The browser fixture was serialized by the Go planner's synthetic media
// canary. Keep it bound to the real contract, not a parallel UI-only schema.
func TestStudioFullDemoFixture(t *testing.T) {
	b, err := os.ReadFile("../../web/lib/full-demo-plan.fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(b, &snapshot); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	if snapshot.Document.Input.FactsRef != "synthetic/facts.json" {
		t.Fatal("fixture must remain explicitly synthetic")
	}
}
