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
	result, ok, err := storedFullDemoResult(store, id, d)
	if err != nil || !ok {
		return all, err
	}
	missing := []int{}
	for _, round := range d.Rounds {
		covered, err := result.fullDemoRoundReusable(store, id, d, round)
		if err != nil {
			return nil, err
		}
		if !covered {
			missing = append(missing, round.Number)
		}
	}
	return missing, nil
}

func MissingFullDemoCameras(store captureStore, id uuid.UUID, d recapplan.Document) ([]string, []string, error) {
	if d.SecondCamera == nil {
		return nil, nil, nil
	}
	result, ok, err := storedFullDemoResult(store, id, d)
	if err != nil {
		return nil, nil, err
	}
	usable := result.UsableFullDemoCameras(d)
	missing, omitted := []string{}, []string{}
	for _, w := range d.SecondCamera.Windows {
		attempted := false
		if i := slices.IndexFunc(d.Rounds, func(r recapplan.Round) bool { return r.ID == w.RoundID }); ok && i >= 0 {
			covered, err := result.fullDemoRoundReusable(store, id, d, d.Rounds[i])
			if err != nil {
				return nil, nil, err
			}
			stored, err := result.fullDemoClipStored(store, id, w.ID)
			if err != nil {
				return nil, nil, err
			}
			attempted = covered && stored && result.storedCamera(w)
		}
		switch {
		case !attempted:
			missing = append(missing, w.ID)
		case !usable[w.ID]:
			omitted = append(omitted, w.ID)
		}
	}
	return missing, omitted, nil
}

func storedFullDemoResult(store captureStore, id uuid.UUID, d recapplan.Document) (RecordingResult, bool, error) {
	exists, err := store.Exists(ResultArtifactKey(id))
	if err != nil || !exists {
		return RecordingResult{}, false, err
	}
	rc, err := store.Open(ResultArtifactKey(id))
	if err != nil {
		return RecordingResult{}, false, err
	}
	defer rc.Close()
	var result RecordingResult
	if json.NewDecoder(rc).Decode(&result) != nil || !result.reusableFullDemoBase(d) {
		return RecordingResult{}, false, nil
	}
	return result, true, nil
}

func (r RecordingResult) fullDemoRoundReusable(store captureStore, id uuid.UUID, d recapplan.Document, round recapplan.Round) (bool, error) {
	want := RecordingSegment{ID: round.ID, TickStart: round.CaptureStartTick, TickEnd: round.CaptureEndTick, LiveEndTick: round.LiveEndTick}
	stored, err := r.fullDemoClipStored(store, id, round.ID)
	if err != nil {
		return false, err
	}
	return stored && r.FullDemoRoundCovered(want, d.Options.Editorial.AllowSafeTailTrim), nil
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
