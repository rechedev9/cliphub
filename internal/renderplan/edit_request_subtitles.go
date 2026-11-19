package renderplan

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// IsZero reports an empty request; SubtitlesHidden makes EditRequest incomparable with ==.
func (r EditRequest) IsZero() bool {
	return reflect.DeepEqual(r, EditRequest{})
}

// MaxHiddenSubtitles bounds the phrases one video hides.
const MaxHiddenSubtitles = 2000

// normalizeSubtitles clears the subtitle options when they are off and keeps one canonical ID per hidden phrase.
func normalizeSubtitles(req EditRequest) EditRequest {
	if !req.Subtitles {
		req.SubtitlesHidden = nil
		return req
	}
	var hidden []string
	for _, raw := range req.SubtitlesHidden {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id != "" && !slices.Contains(hidden, id) {
			hidden = append(hidden, id)
		}
	}
	slices.Sort(hidden)
	req.SubtitlesHidden = hidden
	return req
}

// validateSubtitles keeps subtitles on a video that plays the team comms they transcribe.
func (r EditRequest) validateSubtitles() error {
	if !r.Subtitles {
		if len(r.SubtitlesHidden) > 0 {
			return fmt.Errorf("subtitles_hidden needs subtitles")
		}
		return nil
	}
	switch {
	case r.FullDemo != nil:
		return fmt.Errorf("a Full Demo takes its subtitles from the approved plan")
	case !r.VoiceComms:
		return fmt.Errorf("subtitles need the team comms in the video")
	case len(r.SubtitlesHidden) > MaxHiddenSubtitles:
		return fmt.Errorf("a video hides at most %d phrases", MaxHiddenSubtitles)
	}
	for _, id := range r.SubtitlesHidden {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("subtitles_hidden must list phrase IDs")
		}
	}
	return nil
}
