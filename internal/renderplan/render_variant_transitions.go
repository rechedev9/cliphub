package renderplan

import "time"

func (s *RenderVariantState) Interrupt(message string, at time.Time) {
	s.Status = RenderVariantStatusFailed
	s.Error = message
	s.UpdatedAt = at.UTC()
}

func (s *RenderVariantState) PromoteReady(warnings []string, at time.Time) {
	s.Status = RenderVariantStatusReady
	s.Warnings = nil
	if len(warnings) > 0 {
		s.Warnings = append([]string(nil), warnings...)
	}
	s.UpdatedAt = at.UTC()
}

func (s *RenderVariantState) KeepPublishedArtifacts(previous *RenderVariantState) {
	if s == nil || previous == nil {
		return
	}
	if previous.Status != RenderVariantStatusReady && previous.Status != RenderVariantStatusReview {
		return
	}
	s.EditDocumentKey = previous.EditDocumentKey
	s.EditManifestKey = previous.EditManifestKey
	s.RenderResultKey = previous.RenderResultKey
	s.PackManifestKey = previous.PackManifestKey
	s.GalleryKey = previous.GalleryKey
	s.PublishSummaryKey = previous.PublishSummaryKey
	if previous.ArtifactPrefix != "" {
		s.ArtifactPrefix = previous.ArtifactPrefix
	}
}
