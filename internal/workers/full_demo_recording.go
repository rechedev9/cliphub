package workers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/storage"
)

// Full Demo publishes through the existing recording result pointer, but its
// clips are immutable. Failed replacements cannot invalidate an earlier pack.
func uploadFullDemoRecordingOutputs(store storage.Storage, id uuid.UUID, outDir, resultPath string, attempt, durable recording.RecordingResult, hasPrevious bool) ([]string, error) {
	for i := range durable.Artifacts {
		if isSegmentClip(durable.Artifacts[i]) && durable.Artifacts[i].CaptureRevision == "" {
			durable.Artifacts[i].CaptureRevision = attempt.CaptureRevision
		}
	}
	preserve := func(err error) error {
		if hasPrevious {
			return &recordingCommitPreservedError{cause: err}
		}
		return err
	}
	if err := recording.ValidateUploadResult(durable); err != nil {
		return nil, preserve(err)
	}
	targets, err := recording.NewUploadTargets(recording.NewUploadTargetsOptions{JobID: id, OutDir: outDir, ResultPath: resultPath, Result: attempt})
	if err != nil {
		return nil, preserve(err)
	}
	keys := []string{}
	for _, target := range targets[:len(targets)-1] {
		if exists, err := store.Exists(target.Key); err != nil {
			return nil, preserve(err)
		} else if exists {
			return nil, preserve(fmt.Errorf("capture revision already exists: %s", target.Key))
		}
		if err := uploadFile(store, target.Key, target.Path); err != nil {
			return nil, preserve(err)
		}
		keys = append(keys, target.Key)
		if target.SegmentID == "" {
			durable.ScriptStorageKey = target.Key
			continue
		}
		for i := range durable.Artifacts {
			if durable.Artifacts[i].SegmentID == target.SegmentID && isSegmentClip(durable.Artifacts[i]) {
				durable.Artifacts[i].StorageKey = target.Key
			}
		}
	}
	durable.PublicationPending = false
	b, err := json.MarshalIndent(durable, "", "  ")
	if err != nil {
		return nil, preserve(err)
	}
	revisionKey, err := durable.RevisionResultKey(id)
	if err != nil {
		return nil, preserve(err)
	}
	if err := store.Put(revisionKey, bytes.NewReader(b)); err != nil {
		return nil, preserve(err)
	}
	keys = append(keys, revisionKey)
	if err := store.Put(recording.ResultArtifactKey(id), bytes.NewReader(b)); err != nil {
		return nil, preserve(err)
	}
	return append(keys, recording.ResultArtifactKey(id)), nil
}

func mergeFullDemoCaptureRuns(previous, next, merged recording.RecordingResult) (recording.RecordingResult, error) {
	for _, original := range []recording.RecordingResult{previous, next} {
		if err := recording.ValidateRunResult(original); err != nil {
			return merged, fmt.Errorf("merge Full Demo source: %w", err)
		}
	}
	nextIDs := map[string]bool{}
	for _, segment := range next.Plan.Segments {
		nextIDs[segment.ID] = true
	}
	used := map[string]bool{}
	for i := range merged.Artifacts {
		artifact := &merged.Artifacts[i]
		if !isSegmentClip(*artifact) {
			continue
		}
		if nextIDs[artifact.SegmentID] {
			artifact.CaptureRevision = next.CaptureRevision
		} else if artifact.CaptureRevision == "" {
			artifact.CaptureRevision = previous.CaptureRevision
		}
		used[artifact.CaptureRevision] = true
	}
	merged.FullDemoRuns = []recording.FullDemoCaptureRun{}
	merged.Plan.FullDemoSources = []recapplan.Document{}
	documents := map[string]bool{}
	mainHash, err := merged.Plan.FullDemo.CaptureHash()
	if err != nil {
		return merged, err
	}
	documents[mainHash] = true
	evidence := *next.FullDemoEvidence
	evidence.CertifiedEnds = map[string]int{}
	merged.FullDemoEvidence = &evidence
	runs := append(previous.CaptureRuns(), next.CaptureRuns()...)
	for _, run := range runs {
		if !used[run.Revision] {
			continue
		}
		delete(used, run.Revision)
		merged.FullDemoRuns = append(merged.FullDemoRuns, run)
		hash, err := run.Plan.FullDemo.CaptureHash()
		if err != nil {
			return merged, err
		}
		if !documents[hash] {
			merged.Plan.FullDemoSources = append(merged.Plan.FullDemoSources, *run.Plan.FullDemo)
			documents[hash] = true
		}
		for _, artifact := range merged.Artifacts {
			if isSegmentClip(artifact) && artifact.CaptureRevision == run.Revision {
				merged.FullDemoEvidence.CertifiedEnds[artifact.SegmentID] = run.Evidence.CertifiedEnds[artifact.SegmentID]
			}
		}
	}
	if len(used) > 0 {
		return merged, fmt.Errorf("full demo reused clips lack an immutable capture origin")
	}
	return merged, nil
}
