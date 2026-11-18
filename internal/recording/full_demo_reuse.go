package recording

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// FullDemoRoundCovered reports whether the stored capture of want.ID spans
// want's window with valid frames; the caller checks the clip file and digest.
func (r RecordingResult) FullDemoRoundCovered(want RecordingSegment, allowSafeTailTrim bool) bool {
	i := slices.IndexFunc(r.Plan.Segments, func(s RecordingSegment) bool { return s.ID == want.ID })
	if i < 0 || r.FullDemoEvidence == nil {
		return false
	}
	stored := r.Plan.Segments[i]
	certifiedEnd := r.FullDemoEvidence.CertifiedEnds[want.ID]
	if stored.TickStart > want.TickStart || stored.TickEnd < want.TickEnd || certifiedEnd < min(want.LiveEndTick+1, want.TickEnd) {
		return false
	}
	if !allowSafeTailTrim && certifiedEnd < want.TickEnd {
		return false
	}
	return r.ValidateFullDemoRoundFrames(want.ID, want.TickStart, min(want.TickEnd, certifiedEnd)) == nil
}

type captureStore interface {
	Open(key string) (io.ReadCloser, error)
	Exists(key string) (bool, error)
}

// MissingFullDemoRounds lists the source rounds of d the stored capture cannot
// supply. It predicts the record worker's reuse without hashing; the worker decides.
func MissingFullDemoRounds(store captureStore, id uuid.UUID, d recapplan.Document) ([]int, error) {
	all := make([]int, 0, len(d.Rounds))
	for _, round := range d.Rounds {
		all = append(all, round.Number)
	}
	exists, err := store.Exists(ResultArtifactKey(id))
	if err != nil || !exists {
		return all, err
	}
	rc, err := store.Open(ResultArtifactKey(id))
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var result RecordingResult
	if json.NewDecoder(rc).Decode(&result) != nil || !result.reusableFullDemoBase(d) {
		return all, nil
	}
	missing := []int{}
	for _, round := range d.Rounds {
		want := RecordingSegment{ID: round.ID, TickStart: round.CaptureStartTick, TickEnd: round.CaptureEndTick, LiveEndTick: round.LiveEndTick}
		stored, err := result.fullDemoClipStored(store, id, round.ID)
		if err != nil {
			return nil, err
		}
		if !stored || !result.FullDemoRoundCovered(want, d.Options.Editorial.AllowSafeTailTrim) {
			missing = append(missing, round.Number)
		}
	}
	return missing, nil
}

// reusableFullDemoBase is the planner's capture identity without the round
// windows: same demo, player, clock, capture options and crosshairs.
func (r RecordingResult) reusableFullDemoBase(d recapplan.Document) bool {
	stored := r.Plan.FullDemo
	if r.Error != "" || r.PublicationPending || r.CaptureMode != CaptureModeReal || !r.CaptureVerified ||
		r.Plan.CaptureContract != CaptureContractVersion || stored == nil {
		return false
	}
	return stored.Input.DemoSHA256 == d.Input.DemoSHA256 && stored.Input.TargetSteamID64 == d.Input.TargetSteamID64 &&
		stored.Clock == d.Clock && stored.Options.Capture == d.Options.Capture && slices.Equal(stored.Crosshairs, d.Crosshairs)
}

func (r RecordingResult) fullDemoClipStored(store captureStore, id uuid.UUID, segmentID string) (bool, error) {
	i := slices.IndexFunc(r.Artifacts, func(a RecordingArtifact) bool {
		return a.Role == "segment" && a.Type == "video" && a.SegmentID == segmentID
	})
	if i < 0 || len(r.Artifacts[i].ContentSHA256) != 64 {
		return false, nil
	}
	key, err := r.SegmentClipKey(id, segmentID)
	if err != nil {
		return false, nil
	}
	return store.Exists(key)
}
