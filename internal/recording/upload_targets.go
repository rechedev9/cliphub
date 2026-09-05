package recording

import (
	"path/filepath"

	"github.com/google/uuid"
)

// UploadTarget maps one local recorder output to its durable storage key.
type UploadTarget struct {
	Key            string
	Path           string
	Label          string
	Required       bool
	MissingMessage string
	SegmentID      string
}

// NewUploadTargetsOptions carries the recorder result and local paths needed
// to plan durable recording uploads.
type NewUploadTargetsOptions struct {
	JobID      uuid.UUID
	OutDir     string
	ResultPath string
	Result     RecordingResult
}

// NewUploadTargets returns the ordered upload plan for one recording run.
func NewUploadTargets(opts NewUploadTargetsOptions) ([]UploadTarget, error) {
	scriptPath := opts.Result.Script
	if scriptPath == "" {
		scriptPath = filepath.Join(opts.OutDir, "recording.js")
	}
	scriptRequired := opts.Result.Error == ""
	missingScriptMessage := ""
	if scriptRequired {
		missingScriptMessage = "recording script not found at " + scriptPath
	}
	scriptKey, err := opts.Result.ScriptKey(opts.JobID)
	if err != nil {
		return nil, err
	}
	targets := []UploadTarget{{
		Key:            scriptKey,
		Path:           scriptPath,
		Label:          "recording script",
		Required:       scriptRequired,
		MissingMessage: missingScriptMessage,
	}}

	for _, artifact := range opts.Result.Artifacts {
		if !isUsableSegmentClip(artifact) {
			continue
		}
		key, err := opts.Result.SegmentClipKey(opts.JobID, artifact.SegmentID)
		if err != nil {
			return nil, err
		}
		targets = append(targets, UploadTarget{
			Key:       key,
			Path:      artifact.Path,
			Label:     "segment " + artifact.SegmentID,
			Required:  true,
			SegmentID: artifact.SegmentID,
		})
	}
	targets = append(targets, UploadTarget{
		Key:      ResultArtifactKey(opts.JobID),
		Path:     opts.ResultPath,
		Label:    "recording result",
		Required: true,
	})
	return targets, nil
}
