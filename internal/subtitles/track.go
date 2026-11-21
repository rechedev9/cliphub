package subtitles

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// Phrase is one transcribed phrase on the demo's tick clock, as the render worker hands it over.
type Phrase struct {
	ID        string       `json:"id"`
	Speaker   string       `json:"speaker"`
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
