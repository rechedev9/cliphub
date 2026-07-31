package renderplan

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/editor"
	"github.com/rechedev9/tickcut/internal/recording"
)

const QualityReportSchemaVersion = "1.0"

type QualityReport struct {
	SchemaVersion string        `json:"schema_version"`
	JobID         uuid.UUID     `json:"job_id"`
	Variant       string        `json:"variant"`
	Status        string        `json:"status"`
	Items         []QualityItem `json:"items"`
	Warnings      []string      `json:"warnings,omitempty"`
	Error         string        `json:"error,omitempty"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

type QualityItem struct {
	SegmentID       string   `json:"segment_id"`
	Status          string   `json:"status"`
	VideoWidth      int      `json:"video_width,omitempty"`
	VideoHeight     int      `json:"video_height,omitempty"`
	DurationSeconds float64  `json:"duration_seconds,omitempty"`
	VideoCodec      string   `json:"video_codec,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

func NewQualityReport(jobID uuid.UUID, variant string, result editor.Result) QualityReport {
	report := QualityReport{
		SchemaVersion: QualityReportSchemaVersion,
		JobID:         jobID,
		Variant:       variant,
		Items:         make([]QualityItem, 0, len(result.Shorts)),
		Warnings:      append([]string(nil), result.Warnings...),
		Error:         result.Error,
		GeneratedAt:   time.Now().UTC(),
	}
	for _, short := range result.Shorts {
		report.Items = append(report.Items, qualityItem(short, result.OutputFormat))
	}
	report.Status = summarizeQuality(report.Items, report.Warnings, result.Error)
	return report
}

// CompleteRenderWarnings merges renderer warnings with every artifact-level QA
// warning used by the quality endpoint. The returned strings are stable review
// tokens, so publication state and the quality report cannot disagree about an
// unusable or incorrectly shaped video.
func CompleteRenderWarnings(result editor.Result) []string {
	warnings := append([]string(nil), result.Warnings...)
	seen := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		seen[warning] = struct{}{}
	}
	for _, short := range result.Shorts {
		artifact := short.PublishArtifact
		if isEmptyArtifact(artifact) {
			artifact = short.OutputArtifact
		}
		// Legacy render documents may predate artifact metadata entirely. That
		// is unknown evidence, not proof that the already-persisted video is
		// empty. Fresh editor results always include an artifact path.
		if isEmptyArtifact(artifact) {
			continue
		}
		for _, code := range artifactWarnings(artifact, effectiveShortOutputFormat(short, result.OutputFormat)) {
			warning := fmt.Sprintf("quality %s: %s", short.SegmentID, code)
			if _, exists := seen[warning]; exists {
				continue
			}
			warnings = append(warnings, warning)
			seen[warning] = struct{}{}
		}
	}
	return warnings
}

func qualityItem(short editor.ShortResult, resultOutputFormat string) QualityItem {
	artifact := short.PublishArtifact
	if isEmptyArtifact(artifact) {
		artifact = short.OutputArtifact
	}
	if isEmptyArtifact(artifact) {
		return QualityItem{
			SegmentID: short.SegmentID,
			Status:    "unknown",
		}
	}
	item := QualityItem{
		SegmentID:       short.SegmentID,
		VideoWidth:      artifact.Width,
		VideoHeight:     artifact.Height,
		DurationSeconds: artifact.DurationSeconds,
		VideoCodec:      artifact.Codec,
		Warnings:        artifactWarnings(artifact, effectiveShortOutputFormat(short, resultOutputFormat)),
	}
	if len(item.Warnings) > 0 {
		item.Status = "warning"
	} else {
		item.Status = "ready"
	}
	return item
}

func effectiveShortOutputFormat(short editor.ShortResult, resultOutputFormat string) string {
	if short.OutputFormat != "" {
		return short.OutputFormat
	}
	return resultOutputFormat
}

func isEmptyArtifact(artifact recording.RecordingArtifact) bool {
	return artifact.Path == "" &&
		artifact.SizeBytes == 0 &&
		artifact.Width == 0 &&
		artifact.Height == 0 &&
		artifact.DurationSeconds == 0 &&
		artifact.ProbeError == ""
}

func artifactWarnings(artifact recording.RecordingArtifact, outputFormat string) []string {
	var warnings []string
	if artifact.ProbeError != "" {
		warnings = append(warnings, "probe_error")
	}
	if artifact.SizeBytes == 0 {
		warnings = append(warnings, "missing_or_empty_video")
	}
	wantWidth, wantHeight := 1080, 1920
	if outputFormat == editor.OutputFormatLandscape16x9 {
		wantWidth, wantHeight = 1920, 1080
	}
	if artifact.Width > 0 && artifact.Height > 0 && (artifact.Width != wantWidth || artifact.Height != wantHeight) {
		warnings = append(warnings, "unexpected_output_resolution")
	}
	if outputFormat != editor.OutputFormatLandscape16x9 && artifact.DurationSeconds > 180 {
		warnings = append(warnings, "too_long_for_shorts")
	}
	return warnings
}

func summarizeQuality(items []QualityItem, resultWarnings []string, resultError string) string {
	if resultError != "" {
		return "failed"
	}
	if len(resultWarnings) > 0 {
		return "warning"
	}
	if len(items) == 0 {
		return "unknown"
	}
	status := "ready"
	for _, item := range items {
		if item.Status == "warning" {
			return "warning"
		}
		if item.Status != "ready" {
			status = "unknown"
		}
	}
	return status
}
