package recording

import (
	"encoding/json"

	"github.com/google/uuid"
)

// ShortClips asks which recorded plays of a job a capture in this HUD and kill feed framing can reuse.
type ShortClips struct {
	JobID                uuid.UUID
	SegmentIDs           []string
	HUDMode              HUDMode
	PortraitSafeKillfeed bool
}

// MissingShortClips lists the segments the stored capture cannot supply. It predicts the record
// worker's reuse without hashing or comparing windows; the worker decides.
func MissingShortClips(store captureStore, want ShortClips) ([]string, error) {
	all := append([]string{}, want.SegmentIDs...)
	exists, err := store.Exists(ResultArtifactKey(want.JobID))
	if err != nil || !exists {
		return all, err
	}
	rc, err := store.Open(ResultArtifactKey(want.JobID))
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var result RecordingResult
	if json.NewDecoder(rc).Decode(&result) != nil || !result.reusableShortBase(want) {
		return all, nil
	}
	missing := []string{}
	for _, id := range want.SegmentIDs {
		stored, err := result.shortClipStored(store, want.JobID, id)
		if err != nil {
			return nil, err
		}
		if !stored {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

func (r RecordingResult) reusableShortBase(want ShortClips) bool {
	return r.Error == "" && !r.PublicationPending && r.CaptureMode == CaptureModeReal && r.CaptureVerified &&
		r.Plan.CaptureContract == CaptureContractVersion && r.Plan.FullDemo == nil &&
		r.Plan.Stream.HUDMode == want.HUDMode && r.Plan.Stream.PortraitSafeKillfeed == want.PortraitSafeKillfeed
}

func (r RecordingResult) shortClipStored(store captureStore, id uuid.UUID, segmentID string) (bool, error) {
	for _, a := range r.Artifacts {
		if a.Role != "segment" || a.Type != "video" || a.SegmentID != segmentID {
			continue
		}
		key, err := r.SegmentClipKey(id, segmentID)
		if err != nil {
			return false, nil
		}
		return store.Exists(key)
	}
	return false, nil
}
