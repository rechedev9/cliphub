package streamclips

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// retiredPlanFields and retiredClipFields are the JSON keys that the removal of
// the stream killfeed and burned-caption pipelines (commit 77a9534) deleted
// from EditPlan and ClipRange. Plans persisted before that removal still carry
// them on disk, and the promise made when the pipelines were deleted is that
// such a plan keeps loading and simply renders without killfeed and without
// subtitles.
//
// Callers that decode a plan strictly must drop exactly these keys and nothing
// else, so a mistyped or invented key still fails loudly instead of being
// ignored. Once no persisted plan predates the removal, both lists and
// DecodeEditPlan's stripping step can be deleted and the strict decode used on
// the raw bytes again.
var (
	retiredPlanFields = []string{
		"killfeed_crop",
		"killfeed_analysis",
		"captions",
	}
	retiredClipFields = []string{
		"killfeed_seconds",
		"killfeed_kills",
		"killfeed_cue_provenance",
		"caption_words",
		"caption_reviewed",
	}
)

// DecodeEditPlan decodes exactly one persisted edit plan from body. Unknown
// fields are rejected so a typo in a saved plan is reported rather than
// silently ignored, except for the keys retired by the killfeed and
// burned-caption removal, which are dropped first.
func DecodeEditPlan(body []byte) (EditPlan, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var document map[string]json.RawMessage
	if err := decoder.Decode(&document); err != nil {
		return EditPlan{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return EditPlan{}, err
	}
	stripped, err := dropRetiredEditPlanFields(document)
	if err != nil {
		return EditPlan{}, err
	}
	var plan EditPlan
	strict := json.NewDecoder(bytes.NewReader(stripped))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&plan); err != nil {
		return EditPlan{}, err
	}
	return plan, nil
}

// dropRetiredEditPlanFields removes the retired keys from one decoded plan
// document and re-encodes it for the strict decode. Every other key is
// preserved verbatim so it still reaches DisallowUnknownFields.
func dropRetiredEditPlanFields(document map[string]json.RawMessage) ([]byte, error) {
	for _, field := range retiredPlanFields {
		delete(document, field)
	}
	if rawClips, present := document["clips"]; present {
		var clips []map[string]json.RawMessage
		if err := json.Unmarshal(rawClips, &clips); err != nil {
			return nil, fmt.Errorf("decode clips: %w", err)
		}
		for _, clip := range clips {
			for _, field := range retiredClipFields {
				delete(clip, field)
			}
		}
		encodedClips, err := json.Marshal(clips)
		if err != nil {
			return nil, fmt.Errorf("encode clips: %w", err)
		}
		document["clips"] = encodedClips
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode stream edit plan: %w", err)
	}
	return encoded, nil
}
