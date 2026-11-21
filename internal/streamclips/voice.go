package streamclips

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

var ErrVoiceLineNotFound = errors.New("stream frase not found")

const SummaryVideo = "summary"

type VoiceTranscript struct {
	JobID     uuid.UUID           `json:"job_id"`
	State     string              `json:"state"`
	Model     string              `json:"model,omitempty"`
	Error     string              `json:"error,omitempty"`
	Spans     []library.VoiceSpan `json:"spans"`
	Pending   []library.VoiceSpan `json:"pending"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type VoiceLine struct {
	ID           uuid.UUID               `json:"id"`
	JobID        uuid.UUID               `json:"job_id"`
	StartMS      int64                   `json:"start_ms"`
	EndMS        int64                   `json:"end_ms"`
	Text         string                  `json:"text"`
	Corrected    bool                    `json:"corrected"`
	OriginalText string                  `json:"original_text,omitempty"`
	Discarded    bool                    `json:"discarded"`
	Words        []library.DemoVoiceWord `json:"-"`
}

type VoiceUpdate struct {
	JobID      uuid.UUID
	Transcript VoiceTranscript
	Replaced   []library.VoiceSpan
	Lines      *[]VoiceLine
}

type VoiceLinePatch struct {
	JobID     uuid.UUID
	ID        uuid.UUID
	Text      *string
	Discarded *bool
}

type Subtitles struct {
	JobID   uuid.UUID              `json:"job_id"`
	Shorts  bool                   `json:"shorts"`
	Summary bool                   `json:"summary"`
	Hidden  map[string][]uuid.UUID `json:"hidden"`
}

func MergeVoiceSpans(spans []library.VoiceSpan) []library.VoiceSpan {
	sorted := make([]library.VoiceSpan, 0, len(spans))
	for _, s := range spans {
		if s.EndMS > s.StartMS {
			sorted = append(sorted, s)
		}
	}
	sortVoiceSpans(sorted)
	out := []library.VoiceSpan{}
	for _, s := range sorted {
		if n := len(out); n > 0 && s.StartMS <= out[n-1].EndMS {
			out[n-1].EndMS = max(out[n-1].EndMS, s.EndMS)
			continue
		}
		out = append(out, s)
	}
	return out
}

func MissingVoiceSpans(wanted, covered []library.VoiceSpan) []library.VoiceSpan {
	out := []library.VoiceSpan{}
	have := MergeVoiceSpans(covered)
	for _, w := range MergeVoiceSpans(wanted) {
		from := w.StartMS
		for _, c := range have {
			if c.EndMS <= from || c.StartMS >= w.EndMS {
				continue
			}
			if c.StartMS > from {
				out = append(out, library.VoiceSpan{StartMS: from, EndMS: c.StartMS})
			}
			from = max(from, c.EndMS)
		}
		if from < w.EndMS {
			out = append(out, library.VoiceSpan{StartMS: from, EndMS: w.EndMS})
		}
	}
	return out
}

func InVoiceSpans(ms int64, spans []library.VoiceSpan) bool {
	for _, s := range spans {
		if ms >= s.StartMS && ms < s.EndMS {
			return true
		}
	}
	return false
}

func sortVoiceSpans(spans []library.VoiceSpan) {
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].StartMS < spans[j-1].StartMS; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
}
