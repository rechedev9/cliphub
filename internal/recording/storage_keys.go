package recording

import (
	"fmt"
	"github.com/google/uuid"
	"path"
	"strings"

	"github.com/rechedev9/cliphub/internal/artifacts"
)

func revisionKey(jobID uuid.UUID, revision, suffix string) (string, error) {
	id, err := uuid.Parse(revision)
	if err != nil || id == uuid.Nil || id.String() != revision {
		return "", fmt.Errorf("invalid recording revision")
	}
	prefix := artifacts.FullDemoCapturePrefix(jobID, id)
	return path.Join(prefix, suffix), nil
}

func validateRevisionKey(jobID uuid.UUID, key, suffix string) error {
	prefix := "jobs/" + jobID.String() + "/recording/revisions/"
	if !strings.HasPrefix(key, prefix) {
		return fmt.Errorf("recording artifact is outside its job revision namespace")
	}
	parts := strings.SplitN(strings.TrimPrefix(key, prefix), "/", 2)
	if len(parts) != 2 || parts[1] != suffix {
		return fmt.Errorf("recording artifact has an invalid revision suffix")
	}
	expected, err := revisionKey(jobID, parts[0], suffix)
	if err != nil || expected != key {
		return fmt.Errorf("recording artifact has an invalid revision key")
	}
	return nil
}

// ScriptKey and SegmentClipKey resolve immutable capture references while
// retaining canonical keys for recordings produced before editorial profiles.
func (r RecordingResult) ScriptKey(jobID uuid.UUID) (string, error) {
	if r.ScriptStorageKey != "" {
		if err := validateRevisionKey(jobID, r.ScriptStorageKey, "recording.js"); err != nil {
			return "", err
		}
		return r.ScriptStorageKey, nil
	}
	if r.Plan.FullDemo != nil {
		return revisionKey(jobID, r.CaptureRevision, "recording.js")
	}
	return ScriptArtifactKey(jobID), nil
}

func (r RecordingResult) SegmentClipKey(jobID uuid.UUID, segmentID string) (string, error) {
	if err := artifacts.ValidateArtifactToken("segment", segmentID); err != nil {
		return "", err
	}
	suffix := "segments/" + segmentID + ".mp4"
	for _, a := range r.Artifacts {
		if a.SegmentID == segmentID && a.Role == "segment" && a.StorageKey != "" {
			if err := validateRevisionKey(jobID, a.StorageKey, suffix); err != nil {
				return "", err
			}
			return a.StorageKey, nil
		}
	}
	if r.Plan.FullDemo != nil {
		return revisionKey(jobID, r.CaptureRevision, suffix)
	}
	return SegmentClipArtifactKey(jobID, segmentID)
}

func (r RecordingResult) RevisionResultKey(jobID uuid.UUID) (string, error) {
	return revisionKey(jobID, r.CaptureRevision, "recording-result.json")
}

// ResultArtifactKey returns the durable recorder result JSON key for a job.
func ResultArtifactKey(jobID uuid.UUID) string {
	return artifacts.RecordingResultKey(jobID)
}

// ScriptArtifactKey returns the durable HLAE recording script key for a job.
func ScriptArtifactKey(jobID uuid.UUID) string {
	return artifacts.RecordingScriptKey(jobID)
}

// SegmentClipArtifactKey returns the durable MP4 key for one recorded segment.
func SegmentClipArtifactKey(jobID uuid.UUID, segmentID string) (string, error) {
	return artifacts.SegmentClipKey(jobID, segmentID)
}
