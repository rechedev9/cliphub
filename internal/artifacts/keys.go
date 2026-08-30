// Package artifacts defines durable object-storage keys for job outputs.
package artifacts

import (
	"fmt"
	"path"
	"regexp"

	"github.com/google/uuid"
)

var artifactTokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func JobPrefix(id uuid.UUID) string {
	return path.Join("jobs", id.String())
}

func RecordingResultKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "recording", "recording-result.json")
}

func RecordingScriptKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "recording", "recording.js")
}

func SegmentClipKey(id uuid.UUID, segmentID string) (string, error) {
	if err := ValidateArtifactToken("segment id", segmentID); err != nil {
		return "", err
	}
	return path.Join(JobPrefix(id), "recording", "segments", segmentID+".mp4"), nil
}

// CaptureSelectionKey is the storage key for the ordered segment ids the
// in-flight recording run will capture. The record worker overwrites it at the
// start of every record task (last writer wins - it is the in-flight reel), and
// the job poll reads it to scope capture progress to that reel instead of the
// whole kill plan.
func CaptureSelectionKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "recording", "capture-selection.json")
}

// CaptureProgressKey is a non-committing status document for the in-flight
// recorder. It never makes a segment reusable; validated clips are still
// published only with the authoritative recording result.
func CaptureProgressKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "recording", "capture-progress.json")
}

func CompositionResultKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "composition", "composition-result.json")
}

func FinalMP4Key(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "composition", "final.mp4")
}

// MomentsKey returns the durable JSON key for the job's scored moment index.
func MomentsKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "moments", "moments.json")
}

// RosterKey is the storage key for a job's roster scan result.
func RosterKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "roster.json")
}

// RecapPlanKey is the sidecar full-round plan used when match_recap is on.
// The job kill plan stays the Shorts burst windows; this artifact is the
// landscape recap ranges captured by HLAE.
func RecapPlanKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "recap-plan.json")
}

// FullDemoFaceitKey is optional FACEIT enrichment for Full Demo overlays.
// Workers read it as demooverlay.Enrichment JSON and never invent numbers.
func FullDemoFaceitKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "full-demo-faceit.json")
}

// AnticheatKey is the storage key for a job's CheaterDetect analysis. The
// analysis is a side lane on the same demo: it never advances the job status,
// so its progress lives entirely inside this document.
func AnticheatKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "anticheat.json")
}

// ProgressKey is the durable in-flight snapshot for a long job stage. Workers
// overwrite it as they walk ticks, clips, or bytes; the UI polls it so leaving
// a screen and coming back resumes the same percent.
func ProgressKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "progress.json")
}

// AnticheatProgressKey is the CheaterDetect side-lane snapshot. It must not
// share ProgressKey: a screening can run beside a parse of the same demo.
func AnticheatProgressKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "anticheat-progress.json")
}

// TacticalProgressKey is the tactical side-lane snapshot.
func TacticalProgressKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "tactical", "progress.json")
}

// TacticalIndexKey is the storage key for a job's tactical document: the round
// index, its classification, and the descriptor of the position blob.
func TacticalIndexKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "tactical", "tactical.json")
}

// TacticalPositionsKey is the storage key for the sidecar position blob the
// tactical document indexes. It is binary and seekable per round, so readers
// fetch one round instead of the whole match.
func TacticalPositionsKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "tactical", "positions.zvpos")
}

// TacticalStatusKey is the storage key for the tactical readiness document.
func TacticalStatusKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "tactical", "status.json")
}

// GenerateIntentKey is the storage key for a job's one-click generate intent:
// the latest accepted preset, music, and edit shown by the workbench. Record
// tasks carry their own immutable intent rather than reading this mutable view.
func GenerateIntentKey(id uuid.UUID) string {
	return path.Join(JobPrefix(id), "generate-intent.json")
}

// RenderVariantPrefix returns the durable storage prefix for a named render
// variant, such as a vertical Shorts pack or a future mobile render.
func RenderVariantPrefix(id uuid.UUID, variant string) (string, error) {
	if err := ValidateArtifactToken("render variant", variant); err != nil {
		return "", err
	}
	return path.Join(JobPrefix(id), "renders", variant), nil
}

// RenderVariantResultKey returns the JSON result key for a named render
// variant.
func RenderVariantResultKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "render-result.json"), nil
}

// RenderVariantStatusKey returns the durable status document key for a named
// render variant.
func RenderVariantStatusKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "status.json"), nil
}

// RenderVariantEditDocumentKey returns the stable user/edit intent document key
// for a named render variant.
func RenderVariantEditDocumentKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "edit-document.json"), nil
}

// RenderVariantEditManifestKey returns the compiled editor manifest key for a
// named render variant.
func RenderVariantEditManifestKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "edit-manifest.json"), nil
}

// RenderVariantPackManifestKey returns the publish-pack manifest key for a
// named render variant.
func RenderVariantPackManifestKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "pack-manifest.json"), nil
}

// RenderVariantPublishSummaryKey returns the markdown summary key for a named
// render variant.
func RenderVariantPublishSummaryKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "publish-summary.md"), nil
}

// RenderVariantVideoKey returns the MP4 key for one video artifact inside a
// named render variant.
func RenderVariantVideoKey(id uuid.UUID, variant, name string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	if err := ValidateArtifactToken("artifact name", name); err != nil {
		return "", err
	}
	return path.Join(prefix, "videos", name+".mp4"), nil
}

// RenderVariantGalleryKey returns the HTML gallery key for a named render
// variant.
func RenderVariantGalleryKey(id uuid.UUID, variant string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "index.html"), nil
}

// RenderVariantLogKey returns a log artifact key for a named render variant.
func RenderVariantLogKey(id uuid.UUID, variant, name string) (string, error) {
	prefix, err := RenderVariantPrefix(id, variant)
	if err != nil {
		return "", err
	}
	if err := ValidateArtifactToken("log name", name); err != nil {
		return "", err
	}
	return path.Join(prefix, "logs", name+".log"), nil
}

// ValidateArtifactToken rejects values that cannot safely be used as one
// artifact-name path component.
func ValidateArtifactToken(label, value string) error {
	if !artifactTokenPattern.MatchString(value) {
		return fmt.Errorf("invalid %s %q", label, value)
	}
	return nil
}
