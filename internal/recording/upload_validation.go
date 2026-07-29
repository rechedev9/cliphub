package recording

import (
	"fmt"
	"strings"

	"github.com/rechedev9/fragforge/internal/artifacts"
)

// ValidateRunResult returns an error when the recorder wrote a structured
// failure result after the process completed.
func ValidateRunResult(result RecordingResult) error {
	if result.Error != "" {
		return fmt.Errorf("recording result error: %s", result.Error)
	}
	if result.PublicationPending {
		return fmt.Errorf("recording result publication is pending")
	}
	if result.Plan.CaptureContract == LegacyCaptureContractVersion {
		return validateLegacyRunResult(result)
	}
	if result.CaptureMode != CaptureModeReal {
		return fmt.Errorf("recording result capture_mode must be %q", CaptureModeReal)
	}
	if err := result.Plan.Validate(); err != nil {
		return fmt.Errorf("recording result plan: %w", err)
	}
	if !result.CaptureVerified {
		return fmt.Errorf("recording result lacks completed POV verification")
	}
	fingerprint, err := CaptureInputFingerprint(result.Plan)
	if err != nil {
		return err
	}
	if result.CaptureInputFingerprint != fingerprint {
		return fmt.Errorf("recording result capture input fingerprint does not match its plan")
	}
	return nil
}

// validateLegacyRunResult preserves the exact verified V1 contract that shipped
// before capture_mode and capture-input fingerprints were persisted. Requiring
// those fields to be absent prevents a malformed current result from selecting
// this compatibility path. The legacy plan still crosses all V1 validation
// boundaries plus today's safe artifact-token check.
func validateLegacyRunResult(result RecordingResult) error {
	if result.CaptureMode != "" || result.CaptureInputFingerprint != "" {
		return fmt.Errorf("legacy recording result contains fields from a newer capture contract")
	}
	if !result.CaptureVerified {
		return fmt.Errorf("recording result lacks completed POV verification")
	}
	if err := validateLegacyPlan(result.Plan); err != nil {
		return fmt.Errorf("legacy recording result plan: %w", err)
	}
	return nil
}

func validateLegacyPlan(plan RecordingPlan) error {
	if plan.CaptureContract != LegacyCaptureContractVersion {
		return fmt.Errorf("capture_contract must be %q", LegacyCaptureContractVersion)
	}
	if plan.KillPlanSchemaVersion != "" || plan.DemoSHA256 != "" ||
		plan.DemoDurationTicks != 0 || len(plan.EditorialSegmentIDs) != 0 {
		return fmt.Errorf("legacy plan contains fields from a newer capture contract")
	}
	if plan.DemoPath == "" {
		return fmt.Errorf("demo_path is required")
	}
	if plan.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if plan.TargetAccountID == 0 {
		return fmt.Errorf("target_account_id is required")
	}
	accountID, err := AccountIDFromSteamID64(plan.TargetSteamID64)
	if err != nil {
		return fmt.Errorf("target_steamid64: %w", err)
	}
	if plan.TargetAccountID != accountID {
		return fmt.Errorf(
			"target_account_id %d does not match target_steamid64 %q (want %d)",
			plan.TargetAccountID,
			plan.TargetSteamID64,
			accountID,
		)
	}
	if plan.Tickrate <= 0 {
		return fmt.Errorf("tickrate must be positive")
	}
	if len(plan.Segments) == 0 {
		return fmt.Errorf("at least one segment is required")
	}
	if plan.Stream.Mode == "" {
		return fmt.Errorf("stream mode is required")
	}
	if plan.Stream.HUDMode != "" && !plan.Stream.HUDMode.Valid() {
		return fmt.Errorf("stream hud_mode must be %q, %q, or %q", HUDModeGameplay, HUDModeClean, HUDModeDeathnotices)
	}
	if plan.Stream.PortraitSafeKillfeed &&
		plan.Stream.HUDMode != HUDModeGameplay &&
		plan.Stream.HUDMode != HUDModeDeathnotices {
		return fmt.Errorf("stream portrait_safe_killfeed requires hud_mode %q or %q", HUDModeGameplay, HUDModeDeathnotices)
	}
	if plan.Stream.FPS <= 0 || plan.Stream.Width <= 0 || plan.Stream.Height <= 0 {
		return fmt.Errorf("stream fps, width, and height must be positive")
	}
	if plan.Stream.CRF < 1 || plan.Stream.CRF > 51 {
		return fmt.Errorf("stream crf must be between 1 and 51")
	}
	if plan.Stream.DeathnoticeSafeZoneX < 0 || plan.Stream.DeathnoticeSafeZoneX > 1 {
		return fmt.Errorf("stream deathnotice_safe_zone_x must be between 0 and 1")
	}
	if plan.Stream.DeathnoticeSafeZoneY < 0 || plan.Stream.DeathnoticeSafeZoneY > 1 {
		return fmt.Errorf("stream deathnotice_safe_zone_y must be between 0 and 1")
	}
	if plan.Stream.DeathnoticeLifetime < 0 || plan.Stream.DeathnoticeLifetime > 10 {
		return fmt.Errorf("stream deathnotice_lifetime_seconds must be between 0 and 10")
	}
	seen := make(map[string]bool, len(plan.Segments))
	for i, segment := range plan.Segments {
		if err := artifacts.ValidateArtifactToken(fmt.Sprintf("segments[%d].id", i), segment.ID); err != nil {
			return err
		}
		if seen[segment.ID] {
			return fmt.Errorf("duplicate segment id %q", segment.ID)
		}
		seen[segment.ID] = true
		if segment.TickStart <= 0 {
			return fmt.Errorf("segment %s tick_start must be positive", segment.ID)
		}
		if segment.TickEnd <= segment.TickStart {
			return fmt.Errorf("segment %s tick_end must be greater than tick_start", segment.ID)
		}
	}
	return nil
}

// ValidateUploadResult returns an error when a successful recorder result does
// not contain every planned segment clip.
func ValidateUploadResult(result RecordingResult) error {
	if result.Error != "" {
		return nil
	}
	if err := ValidateRunResult(result); err != nil {
		return err
	}
	clips := map[string]bool{}
	for _, artifact := range result.Artifacts {
		if isUsableSegmentClip(artifact) {
			clips[artifact.SegmentID] = true
		}
	}
	if len(clips) == 0 {
		return fmt.Errorf("recording result has no segment clips")
	}

	missing := make([]string, 0, len(result.Plan.Segments))
	for _, segment := range result.Plan.Segments {
		if !clips[segment.ID] {
			missing = append(missing, segment.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("recording result missing segment clips: %s", strings.Join(missing, ", "))
	}
	return nil
}

func isUsableSegmentClip(artifact RecordingArtifact) bool {
	return artifact.Role == "segment" && artifact.Type == "video" && artifact.SegmentID != "" &&
		artifact.Path != "" && artifact.SizeBytes > 0 && artifact.ProbeError == ""
}
