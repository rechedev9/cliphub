package workers

import (
	"context"
	"reflect"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/storage"
)

type recordingReuse struct {
	keys       []string
	replayOnly []string
}

func withoutVictimReplay(segment recording.RecordingSegment) recording.RecordingSegment {
	segment.VictimReplay = nil
	segment.AfterReplays = false
	segment.ReplayOnly = false
	return segment
}

func recordingOutputsReady(ctx context.Context, store storage.Storage, id uuid.UUID, requested []string, expectedPlan recording.RecordingPlan) ([]string, recordingReuse, error) {
	if len(requested) == 0 {
		return nil, recordingReuse{}, nil
	}
	needAll := append([]string(nil), requested...)
	resultKey := recording.ResultArtifactKey(id)
	exists, err := store.Exists(resultKey)
	if err != nil || !exists {
		return needAll, recordingReuse{}, err
	}
	result, err := decodeStoredRecordingResult(store, id)
	if err != nil || result.Error != "" {
		return needAll, recordingReuse{}, err
	}
	if result.PublicationPending {
		return needAll, recordingReuse{}, nil
	}
	if err := recording.ValidateRunResult(result); err != nil {
		return needAll, recordingReuse{}, nil
	}
	if !captureProfilesCompatible(result.Plan, expectedPlan) &&
		!legacyCaptureProfileCompatible(result.Plan, expectedPlan) {
		return needAll, recordingReuse{}, nil
	}

	recorded := make(map[string]bool)
	recordedSegments := make(map[string]recording.RecordingSegment, len(result.Plan.Segments))
	for _, segment := range result.Plan.Segments {
		recordedSegments[segment.ID] = segment
	}
	for _, a := range result.Artifacts {
		if isSegmentClip(a) {
			recorded[a.SegmentID] = true
		}
	}
	scriptKey, err := result.ScriptKey(id)
	if err != nil {
		return needAll, recordingReuse{}, err
	}
	if ok, err := store.Exists(scriptKey); err != nil || !ok {
		return needAll, recordingReuse{}, err
	}
	keys := []string{resultKey, scriptKey}
	expectedSegments := make(map[string]recording.RecordingSegment, len(expectedPlan.Segments))
	for _, segment := range expectedPlan.Segments {
		expectedSegments[segment.ID] = segment
	}

	var missing, replayOnly []string
	for _, segID := range requested {
		storedSegment, stored := recordedSegments[segID]
		expectedSegment, expected := expectedSegments[segID]
		compatibleWindow := reflect.DeepEqual(withoutVictimReplay(storedSegment), withoutVictimReplay(expectedSegment))
		if result.Plan.FullDemo != nil && expectedPlan.FullDemo != nil && stored && expected {
			compatibleWindow = result.FullDemoRoundCovered(expectedSegment, expectedPlan.FullDemo.Options.Editorial.AllowSafeTailTrim)
		}
		if !recorded[segID] || !stored || !expected || !compatibleWindow {
			missing = append(missing, segID)
			continue
		}
		clipKey, err := result.SegmentClipKey(id, segID)
		if err != nil {
			return needAll, recordingReuse{}, err
		}
		ok, err := store.Exists(clipKey)
		if err != nil {
			return needAll, recordingReuse{}, err
		}
		if !ok {
			missing = append(missing, segID)
			continue
		}
		if expectedSegment.VictimReplay != nil {
			replayID := recording.VictimReplayID(segID)
			replayKey, err := result.SegmentClipKey(id, replayID)
			if err != nil {
				return needAll, recordingReuse{}, err
			}
			if ok, err := store.Exists(replayKey); err != nil {
				return needAll, recordingReuse{}, err
			} else if !ok || !recorded[replayID] || !reflect.DeepEqual(storedSegment.VictimReplay, expectedSegment.VictimReplay) {
				missing = append(missing, segID)
				replayOnly = append(replayOnly, segID)
				continue
			}
			keys = append(keys, replayKey)
		}
		if result.Plan.FullDemo != nil {
			if ok, err := storedClipIntact(ctx, store, clipKey, clipDigest(result, segID)); err != nil {
				return needAll, recordingReuse{}, err
			} else if !ok {
				missing = append(missing, segID)
				continue
			}
			cameraKey, ok, err := storedFullDemoCamera(ctx, store, id, result, storedSegment, expectedSegment)
			if err != nil {
				return needAll, recordingReuse{}, err
			}
			if !ok {
				missing = append(missing, segID)
				replayOnly = append(replayOnly, segID)
				continue
			}
			if cameraKey != "" {
				keys = append(keys, cameraKey)
			}
		}
		keys = append(keys, clipKey)
	}
	return missing, recordingReuse{keys: keys, replayOnly: replayOnly}, nil
}

func storedFullDemoCamera(ctx context.Context, store storage.Storage, id uuid.UUID, result recording.RecordingResult, stored, expected recording.RecordingSegment) (string, bool, error) {
	w := expected.SecondCamera
	if w == nil {
		return "", true, nil
	}
	if stored.SecondCamera == nil || !recording.SameCameraWindow(*stored.SecondCamera, *w) {
		return "", false, nil
	}
	key, err := result.SegmentClipKey(id, w.ID)
	if err != nil {
		return "", false, err
	}
	ok, err := storedClipIntact(ctx, store, key, clipDigest(result, w.ID))
	return key, ok, err
}

func clipDigest(result recording.RecordingResult, segmentID string) string {
	for _, artifact := range result.Artifacts {
		if isSegmentClip(artifact) && artifact.SegmentID == segmentID {
			return artifact.ContentSHA256
		}
	}
	return ""
}

func storedClipIntact(ctx context.Context, store storage.Storage, key, digest string) (bool, error) {
	if len(digest) != 64 {
		return false, nil
	}
	if err := mediaassets.VerifyContent(ctx, store, key, digest, 8<<30); err != nil {
		return false, ctx.Err()
	}
	return true, nil
}
