package streamclips

import (
	"encoding/json"
	"testing"
)

// TestLegacyPlanDropsKillfeedAndCaptions pins the compatibility promise of the
// removal: an edit plan persisted while killfeed and burned captions still
// existed keeps loading and validating, and simply renders without them.
func TestLegacyPlanDropsKillfeedAndCaptions(t *testing.T) {
	const legacy = `{
	  "schema_version": "1.0",
	  "source": {"path": "stream.mp4"},
	  "variant": "streamer-vertical-stack-40-60",
	  "face_crop": {"x": 0, "y": 0, "width": 0.25, "height": 0.3},
	  "gameplay_crop": {"x": 0, "y": 0, "width": 1, "height": 1},
	  "killfeed_crop": {"x": 0.7, "y": 0.05, "width": 0.28, "height": 0.2},
	  "killfeed_analysis": {"generation_id": "0f1c9a24-2f21-4a7b-9a52-2ab3f1e0c111", "fingerprint": "b1946ac92492d2347c6235b4d2611184"},
	  "captions": {"enabled": true, "language": "es"},
	  "clips": [{
	    "id": "clip-001",
	    "start_seconds": 10,
	    "end_seconds": 25,
	    "killfeed_seconds": [12.5, 18.25],
	    "killfeed_kills": [[{"attacker": "a", "victim": "b", "weapon": "ak47", "attacker_side": "CT"}], []],
	    "caption_words": [{"word": "hola", "start_seconds": 11, "end_seconds": 11.4}],
	    "caption_reviewed": true
	  }]
	}`

	var plan EditPlan
	if err := json.Unmarshal([]byte(legacy), &plan); err != nil {
		t.Fatalf("decode legacy plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("validate legacy plan: %v", err)
	}
	if len(plan.Clips) != 1 {
		t.Fatalf("clips = %d, want 1", len(plan.Clips))
	}

	// Re-encoding must not carry the dropped keys back out to disk.
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("encode plan: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(encoded, &round); err != nil {
		t.Fatalf("decode round trip: %v", err)
	}
	for _, key := range []string{"killfeed_crop", "killfeed_analysis", "captions"} {
		if _, present := round[key]; present {
			t.Errorf("re-encoded plan still carries %q", key)
		}
	}
}
