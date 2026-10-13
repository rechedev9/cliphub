package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
)

// ErrCloudDemoRejected marks a cloud demo the pipeline will never accept: the
// file is not a CS2 demo. The bridge fails the job instead of retrying it.
var ErrCloudDemoRejected = errors.New("cloud demo rejected")

// CloudDemoAdmission is one claimed ClipHub cloud job as the bridge hands it
// to the local pipeline. TargetSteamID and Rules come from the job spec, so
// the job parses straight away instead of waiting for a pick in Studio.
type CloudDemoAdmission struct {
	Demo           io.Reader
	FileName       string
	CloudRequestID string
	TargetSteamID  string
	Rules          rules.Rules
}

// AdmitCloudDemo is the local cloud bridge's entry point for turning a
// claimed ClipHub Portal job into a normal local Job: the same
// validated-stream-to-storage path CreateJob uses for a manual upload,
// tagged with the originating cloud request id so the bridge can find it
// again. An empty TargetSteamID queues a roster scan, like a manual upload
// without a target.
func (h *Handlers) AdmitCloudDemo(ctx context.Context, in CloudDemoAdmission) (*job.Job, error) {
	// Re-validate the magic bytes here too: the portal already rejects
	// anything that isn't a raw CS2 demo at upload time, but the bridge
	// crosses a network boundary to fetch it, and CreateJob applies this same
	// check to every other admission path.
	var header [8]byte
	n, err := io.ReadFull(in.Demo, header[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, fmt.Errorf("read demo header: %w", err)
	}
	if isCSGODemoHeader(header[:n]) {
		return nil, fmt.Errorf("%w: cloud demo %s is a CS:GO demo; only CS2 demos are supported", ErrCloudDemoRejected, in.CloudRequestID)
	}
	if !isDemoHeader(header[:n]) {
		return nil, fmt.Errorf("%w: cloud demo %s is not a CS2 demo", ErrCloudDemoRejected, in.CloudRequestID)
	}
	if in.TargetSteamID != "" {
		if _, err := strconv.ParseUint(in.TargetSteamID, 10, 64); err != nil {
			return nil, fmt.Errorf("cloud demo %s: target steamid must be a 64-bit unsigned integer", in.CloudRequestID)
		}
	}
	if err := in.Rules.Validate(); err != nil {
		return nil, fmt.Errorf("cloud demo %s: invalid rules: %w", in.CloudRequestID, err)
	}
	demo := io.MultiReader(bytes.NewReader(header[:n]), in.Demo)

	// The name was chosen by a stranger and crossed the portal: it gets the
	// same reduction to a safe display name as a manual upload.
	fileName := sanitizeDemoFileName(in.FileName)
	return h.persistAndEnqueueDemo(ctx, demo, fileName, in.TargetSteamID, "", in.CloudRequestID, in.Rules)
}
