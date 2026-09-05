package recording

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// CaptureInputFingerprint binds a reusable capture to every source, player,
// timeline, and recording-profile value that can affect its pixels. Local
// staging paths and runtime output paths are intentionally excluded.
func CaptureInputFingerprint(plan RecordingPlan) (string, error) {
	input := struct {
		FullDemoSourceHashes  []string           `json:"full_demo_source_hashes,omitempty"`
		FullDemoCrosshairHash string             `json:"full_demo_crosshair_hash,omitempty"`
		CaptureContract       string             `json:"capture_contract"`
		KillPlanSchemaVersion string             `json:"killplan_schema_version"`
		DemoSHA256            string             `json:"demo_sha256"`
		DemoMap               string             `json:"demo_map"`
		DemoDurationTicks     int                `json:"demo_duration_ticks"`
		TargetSteamID64       string             `json:"target_steamid64"`
		TargetNameInDemo      string             `json:"target_name_in_demo"`
		TargetAccountID       uint32             `json:"target_account_id"`
		Tickrate              int                `json:"tickrate"`
		Segments              []RecordingSegment `json:"segments"`
		Stream                StreamConfig       `json:"stream"`
		Runtime               RuntimeConfig      `json:"runtime"`
	}{
		CaptureContract:       plan.CaptureContract,
		KillPlanSchemaVersion: plan.KillPlanSchemaVersion,
		DemoSHA256:            plan.DemoSHA256,
		DemoMap:               plan.DemoMap,
		DemoDurationTicks:     plan.DemoDurationTicks,
		TargetSteamID64:       plan.TargetSteamID64,
		TargetNameInDemo:      plan.TargetNameInDemo,
		TargetAccountID:       plan.TargetAccountID,
		Tickrate:              plan.Tickrate,
		Segments:              plan.Segments,
		Stream:                plan.Stream,
		Runtime:               plan.Runtime,
	}
	if plan.FullDemo != nil {
		var err error
		input.FullDemoCrosshairHash, err = fullDemoCrosshairHash(*plan.FullDemo)
		if err != nil {
			return "", err
		}
		for _, source := range plan.FullDemoSources {
			hash, err := source.CaptureHash()
			if err != nil {
				return "", err
			}
			input.FullDemoSourceHashes = append(input.FullDemoSourceHashes, hash)
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal capture input: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
