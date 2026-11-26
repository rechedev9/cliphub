package httpapi

import (
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
)

type generateRunView struct {
	Variant       string    `json:"variant"`
	Active        bool      `json:"active"`
	FailureReason string    `json:"failure_reason,omitempty"`
	FailureCode   string    `json:"failure_code,omitempty"`
	FailedAt      time.Time `json:"failed_at,omitzero"`
}

func (h *Handlers) generateRunView(id uuid.UUID, status job.Status) *generateRunView {
	if status.CaptureActive() {
		return nil
	}
	intent, found := h.workbenchGenerateIntent(id)
	if !found {
		return nil
	}
	if intent.ActiveRunID != uuid.Nil {
		return &generateRunView{Variant: intent.Variant, Active: true}
	}
	if intent.FailureReason == "" || status != job.StatusRecorded {
		return nil
	}
	return &generateRunView{
		Variant:       intent.Variant,
		FailureReason: intent.FailureReason,
		FailureCode:   jobFailureCode(intent.FailureReason, ""),
		FailedAt:      intent.FailedAt,
	}
}
