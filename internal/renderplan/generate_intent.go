package renderplan

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GenerateIntent is the normalized choice behind a one-click reel generation
// request: preset variant, optional music, mix gains, and edit treatment. Each
// accepted record task carries an immutable copy, while the job-scoped artifact
// fences overlapping capture/render work and mirrors the current choice for
// workbench display. ActiveRunID is non-zero only while that accepted capture
// still owns the guided-flow handoff to a render task.
type GenerateIntent struct {
	Variant     string      `json:"variant"`
	MusicKey    string      `json:"music_key,omitempty"`
	MusicVolume float64     `json:"music_volume,omitempty"`
	GameVolume  *float64    `json:"game_volume,omitempty"`
	Edit        EditRequest `json:"edit"`
	// SegmentIDs, when non-empty, scopes the chained render to exactly these
	// recorded plan segments in this order; empty means every recorded segment
	// (the CLI all-kills default and Full Demo recap).
	SegmentIDs  []string  `json:"segment_ids,omitempty"`
	ActiveRunID uuid.UUID `json:"active_run_id,omitzero"`
	AcceptedAt  time.Time `json:"accepted_at,omitzero"`
}

// Normalize fills unset edit fields with their defaults and returns the result.
func (g GenerateIntent) Normalize() GenerateIntent {
	g.Edit = NormalizeEditRequest(g.Edit)
	return g
}

// Validate reports whether the intent names a known preset variant and carries
// a valid edit request. The music key is validated where the render task is
// built (it shares the render-variant token rules), so it is not checked here.
func (g GenerateIntent) Validate() error {
	if _, err := LoadoutForVariant(g.Variant); err != nil {
		return err
	}
	if g.MusicVolume < 0 || g.MusicVolume > 1 {
		return fmt.Errorf("music volume must be between 0 and 1")
	}
	if g.GameVolume != nil && (*g.GameVolume < 0 || *g.GameVolume > 1) {
		return fmt.Errorf("game volume must be between 0 and 1")
	}
	return g.Edit.Validate()
}
