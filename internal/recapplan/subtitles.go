package recapplan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// MaxHiddenSubtitles bounds the phrases one Full Demo hides.
const MaxHiddenSubtitles = 2000

// SubtitleOptions burn the team comms transcript in the spoken language, leaving out Hidden
// phrase IDs. They only change the render, so turning them on keeps the capture.
type SubtitleOptions struct {
	Enabled bool `json:"enabled"`
	// LegacyEnglish is the retired translation switch of Studio 5.25.0 plans, kept so their hash holds; the render ignores it.
	LegacyEnglish bool     `json:"english,omitempty"`
	Hidden        []string `json:"hidden,omitempty"`
}

// canonicalSubtitles drops subtitles that are off and sorts the hidden IDs, so one choice has one plan hash.
func canonicalSubtitles(s *SubtitleOptions) *SubtitleOptions {
	if s == nil || !s.Enabled {
		return nil
	}
	hidden := make([]string, 0, len(s.Hidden))
	for _, id := range s.Hidden {
		hidden = append(hidden, strings.ToLower(strings.TrimSpace(id)))
	}
	return &SubtitleOptions{Enabled: true, Hidden: slices.Compact(slices.Sorted(slices.Values(hidden)))}
}

func (o Options) validateSubtitles() error {
	s := o.Subtitles
	if s == nil {
		return nil
	}
	switch {
	case !s.Enabled && len(s.Hidden) > 0:
		return fmt.Errorf("subtitles.hidden needs subtitles.enabled")
	case s.Enabled && !o.Audio.Voice.Enabled:
		return fmt.Errorf("subtitles need the team voices in the video")
	case len(s.Hidden) > MaxHiddenSubtitles:
		return fmt.Errorf("a Full Demo hides at most %d phrases", MaxHiddenSubtitles)
	}
	for _, id := range s.Hidden {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("subtitles.hidden must list phrase IDs")
		}
	}
	return nil
}
