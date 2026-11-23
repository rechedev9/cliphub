package subtitles

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/google/uuid"
)

var ErrTranscriptFailed = errors.New("the transcription of the team voices failed")

// Phrase is one transcribed phrase on the demo's tick clock, as the render worker hands it over.
type Phrase struct {
	ID        string       `json:"id"`
	Speaker   string       `json:"speaker"`
	SteamID64 string       `json:"steamid64,omitempty"`
	Slot      int          `json:"slot,omitempty"`
	Text      string       `json:"text"`
	StartTick int          `json:"start_tick"`
	EndTick   int          `json:"end_tick"`
	Words     []PhraseWord `json:"words,omitempty"`
}

type PhraseWord struct {
	Text      string `json:"text"`
	StartTick int    `json:"start_tick"`
	EndTick   int    `json:"end_tick"`
}

// Track is the transcript a video draws; Hidden lists the phrase IDs this video leaves out.
type Track struct {
	Phrases []Phrase `json:"phrases"`
	Hidden  []string `json:"hidden,omitempty"`
}

// Request is what a render asks of its demo's transcript: the phrases of Target's team, leaving out Hidden.
type Request struct {
	JobID  uuid.UUID
	Target string
	Hidden []string
}

type Lookup struct {
	track    *Track
	progress float64
}

func Ready(track Track) Lookup {
	return Lookup{track: &track, progress: 1}
}

func Pending(progress float64) Lookup {
	if math.IsNaN(progress) {
		progress = 0
	}
	return Lookup{progress: min(1, max(0, progress))}
}

func (l Lookup) Track() (*Track, bool) {
	return l.track, l.track != nil
}

func (l Lookup) Progress() float64 {
	return l.progress
}

// Load reads the track the render worker wrote.
func Load(path string) (*Track, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the worker's own file in the render folder.
	if err != nil {
		return nil, fmt.Errorf("read subtitles: %w", err)
	}
	var track Track
	if err := json.Unmarshal(raw, &track); err != nil {
		return nil, fmt.Errorf("decode subtitles: %w", err)
	}
	return &track, nil
}
