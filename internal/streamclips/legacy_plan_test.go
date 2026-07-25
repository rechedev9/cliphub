package streamclips

import (
	"encoding/json"
	"strings"
	"testing"
)

// legacyPlanJSON is an edit plan persisted while killfeed and burned captions
// still existed. It carries every key retired by their removal, at both the
// plan and the clip level.
const legacyPlanJSON = `{
	  "schema_version": "1.0",
	  "variant": "streamer-vertical-stack-40-60",
	  "face_crop": {"x": 0, "y": 0, "width": 0.25, "height": 0.3},
	  "gameplay_crop": {"x": 0, "y": 0, "width": 1, "height": 1},
	  "killfeed_crop": {"x": 0.7, "y": 0.05, "width": 0.28, "height": 0.2},
	  "killfeed_analysis": {"generation_id": "0f1c9a24-2f21-4a7b-9a52-2ab3f1e0c111", "status": "applied"},
	  "captions": {"enabled": true, "language": "es"},
	  "clips": [{
	    "id": "clip-001",
	    "start_seconds": 10,
	    "end_seconds": 25,
	    "killfeed_seconds": [12.5, 18.25],
	    "killfeed_kills": [[{"attacker_name": "a", "victim_name": "b", "weapon": "ak47", "attacker_side": "CT", "victim_side": "T"}], []],
	    "killfeed_cue_provenance": [{"cue_seconds": 12.5, "origin": "automatic"}],
	    "caption_words": [{"word": "hola", "start_seconds": 1, "end_seconds": 1.4}],
	    "caption_reviewed": true
	  }]
	}`

// TestDecodeEditPlanDropsKillfeedAndCaptions pins the compatibility promise of
// the removal on the strict decode the CLI actually uses: an edit plan
// persisted while killfeed and burned captions still existed keeps loading and
// validating, and simply renders without them. The end-to-end promise is
// covered by TestRunStreamRenderAcceptsPlanWithRetiredKillfeedAndCaptionKeys.
func TestDecodeEditPlanDropsKillfeedAndCaptions(t *testing.T) {
	plan, err := DecodeEditPlan([]byte(legacyPlanJSON))
	if err != nil {
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

// TestDecodeEditPlanRejectsUnknownFields keeps the retired-key handling from
// turning into blanket leniency: only the retired keys are dropped.
func TestDecodeEditPlanRejectsUnknownFields(t *testing.T) {
	for name, body := range map[string]string{
		"invented plan key": strings.Replace(legacyPlanJSON, `"captions"`, `"captionz"`, 1),
		"invented clip key": strings.Replace(legacyPlanJSON, `"caption_words"`, `"caption_word"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeEditPlan([]byte(body))
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("DecodeEditPlan error = %v, want an unknown field rejection", err)
			}
		})
	}
}

// TestDecodeEditPlanRejectsNonObjectPlan keeps the decoder's internal document
// type out of the message an operator reads: a plan file that is not a JSON
// object must be reported as such.
func TestDecodeEditPlanRejectsNonObjectPlan(t *testing.T) {
	for name, body := range map[string]string{
		"array":  `[{"variant":"streamer-vertical-stack-40-60"}]`,
		"string": `"not a plan"`,
		"number": `42`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeEditPlan([]byte(body))
			if err == nil {
				t.Fatal("DecodeEditPlan error = nil, want a non-object rejection")
			}
			if !strings.Contains(err.Error(), "must be a JSON object") {
				t.Fatalf("DecodeEditPlan error = %v, want a non-object rejection", err)
			}
			if strings.Contains(err.Error(), "json.RawMessage") {
				t.Fatalf("DecodeEditPlan error = %v, must not expose the internal document type", err)
			}
		})
	}
}

func TestDecodeEditPlanRejectsAdditionalJSONValues(t *testing.T) {
	_, err := DecodeEditPlan([]byte(legacyPlanJSON + "\n{}\n"))
	if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("DecodeEditPlan error = %v, want a multiple JSON values rejection", err)
	}
}
