package httpapi

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
)

// captureRequired refuses a generate sent with allow_capture false, as a page load sends it,
// whose record task would open CS2: only an explicit request may start a capture.
const captureRequired = "capture_required"

// CaptureCheck reports whether a record task would open CS2 instead of reusing every stored clip.
type CaptureCheck interface {
	CaptureNeeded(ctx context.Context, t *asynq.Task) (bool, error)
}

// WithCaptureCheck answers a generate that forbids a capture with the record worker's own reuse decision.
func WithCaptureCheck(check CaptureCheck) Option {
	return func(h *Handlers) {
		h.captureCheck = check
	}
}

// captureNeeded is true unless the record worker proves t reuses every stored clip.
func (h *Handlers) captureNeeded(ctx context.Context, t *asynq.Task) bool {
	if h.captureCheck == nil {
		return true
	}
	needed, err := h.captureCheck.CaptureNeeded(ctx, t)
	if err != nil {
		log.Printf("httpapi: capture check: %v", err)
		return true
	}
	return needed
}
