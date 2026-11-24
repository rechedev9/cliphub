package streamclips

import (
	"slices"
	"time"
)

type AttemptOutcome struct {
	Status    Status
	Warnings  []string
	Error     string
	ErrorCode string
	At        time.Time
}

type PublishedRevision struct {
	ResultKey   string
	GalleryKey  string
	ArtifactDir string
	Videos      []VideoEntry
	Delivery    []DeliveryEntry
	Warnings    []string
	At          time.Time
}

func (s *RenderState) SettleAttempt(outcome AttemptOutcome) {
	s.Status = outcome.Status
	s.Warnings = append([]string(nil), outcome.Warnings...)
	s.Error = outcome.Error
	s.ErrorCode = outcome.ErrorCode
	s.UpdatedAt = outcome.At.UTC()
}

func (s *RenderState) Fail(message string, at time.Time) {
	s.SettleAttempt(AttemptOutcome{Status: StatusFailed, Error: message, At: at})
}

func (s *RenderState) Interrupt(message string, at time.Time) {
	s.Status = StatusFailed
	s.Error = message
	s.UpdatedAt = at.UTC()
}

func (s *RenderState) Publish(revision PublishedRevision) {
	s.SettleAttempt(AttemptOutcome{Status: StatusRendered, Warnings: revision.Warnings, At: revision.At})
	s.Published = true
	s.ResultKey = revision.ResultKey
	s.GalleryKey = revision.GalleryKey
	s.ArtifactDir = revision.ArtifactDir
	s.Videos = append([]VideoEntry(nil), revision.Videos...)
	s.Delivery = append([]DeliveryEntry(nil), revision.Delivery...)
}

func (s *RenderState) RemoveVideo(clipID string, at time.Time) []string {
	isClip := func(v VideoEntry) bool { return v.ClipID == clipID }
	i := slices.IndexFunc(s.Videos, isClip)
	if i < 0 {
		return nil
	}
	keys := []string{s.Videos[i].Key}
	isCopy := func(d DeliveryEntry) bool { return d.Kind == "video" && d.Name == clipID+".mp4" }
	if j := slices.IndexFunc(s.Delivery, isCopy); j >= 0 {
		keys = append(keys, s.Delivery[j].Key)
	}
	s.Videos = slices.DeleteFunc(s.Videos, isClip)
	s.Delivery = slices.DeleteFunc(s.Delivery, isCopy)
	s.UpdatedAt = at.UTC()
	return keys
}

func (s RenderState) ListsVideosOf(result RenderResult) bool {
	if len(s.Videos) == 0 {
		return false
	}
	next := 0
	for _, clip := range result.Clips {
		if next < len(s.Videos) && clip.ClipID == s.Videos[next].ClipID && clip.Key == s.Videos[next].Key {
			next++
		}
	}
	return next == len(s.Videos)
}
