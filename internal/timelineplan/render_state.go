package timelineplan

import (
	"time"

	"github.com/google/uuid"
)

type RenderAttempt struct {
	ProjectID   uuid.UUID
	AttemptID   uuid.UUID
	Fingerprint string
	At          time.Time
}

type PublishedRender struct {
	Fingerprint string
	VideoKey    string
	CoverKey    string
	ResultKey   string
	Warnings    []string
	At          time.Time
}

func (s RenderState) StartAttempt(a RenderAttempt) RenderState {
	return RenderState{
		ProjectID:   a.ProjectID,
		AttemptID:   a.AttemptID,
		Status:      StatusRendering,
		Fingerprint: a.Fingerprint,
		VideoKey:    s.VideoKey,
		CoverKey:    s.CoverKey,
		ResultKey:   s.ResultKey,
		UpdatedAt:   a.At,
	}
}

func (s RenderState) Fail(reason string, at time.Time) RenderState {
	failed := s
	failed.Status = StatusFailed
	failed.Error = reason
	failed.Warnings = nil
	failed.UpdatedAt = at
	return failed
}

func (s RenderState) Publish(r PublishedRender) RenderState {
	published := s
	published.Status = StatusRendered
	published.Fingerprint = r.Fingerprint
	published.VideoKey = r.VideoKey
	published.CoverKey = r.CoverKey
	published.ResultKey = r.ResultKey
	published.Warnings = append([]string(nil), r.Warnings...)
	published.Error = ""
	published.UpdatedAt = r.At
	return published
}
