package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
)

// AdmitCloudDemo is the local cloud bridge's entry point for turning an
// approved ClipHub Portal request into a normal local Job: the same
// validated-stream-to-storage and roster-scan-enqueue path CreateJob uses for
// a manual upload, tagged with the originating cloud request id so the
// bridge can find it again once a render is ready. targetSteamID is always
// left empty — the owner picks the target in Studio exactly as they would
// for a demo they uploaded themselves.
func (h *Handlers) AdmitCloudDemo(ctx context.Context, demo io.Reader, fileName, cloudRequestID string) (*job.Job, error) {
	// Re-validate the magic bytes here too: the portal already rejects
	// anything that isn't a raw CS2/GOTV demo at upload time, but the bridge
	// crosses a network boundary to fetch it, and CreateJob applies this same
	// check to every other admission path.
	var header [8]byte
	n, err := io.ReadFull(demo, header[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, fmt.Errorf("read demo header: %w", err)
	}
	if !isDemoHeader(header[:n]) {
		return nil, fmt.Errorf("cloud demo %s is not a CS2 demo", cloudRequestID)
	}
	demo = io.MultiReader(bytes.NewReader(header[:n]), demo)

	return h.persistAndEnqueueDemo(ctx, demo, fileName, "", "", cloudRequestID, rules.Default())
}
