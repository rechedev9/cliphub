package streamclips

import (
	"sync"

	"github.com/google/uuid"
)

// RenderProgress is the live encode progress of one stream render.
type RenderProgress struct {
	Variant string `json:"variant"`
	// Percent covers every clip of the render, from 0 to 99; publishing the
	// revision is what completes it.
	Percent float64 `json:"percent"`
	// Clip is the 1-based clip FFmpeg is encoding now, out of Clips.
	Clip  int `json:"clip"`
	Clips int `json:"clips"`
}

// NewRenderProgress weighs every clip the same: done finished clips plus the
// share of the current one, over the clip count.
func NewRenderProgress(variant string, done, clips int, clipFraction float64) RenderProgress {
	if clips <= 0 {
		return RenderProgress{Variant: variant}
	}
	done = min(max(done, 0), clips)
	clipFraction = min(max(clipFraction, 0), 1)
	percent := min((float64(done)+clipFraction)/float64(clips)*100, 99)
	return RenderProgress{Variant: variant, Percent: percent, Clip: min(done+1, clips), Clips: clips}
}

// RenderProgressBoard holds the progress of running stream renders. It is in
// memory on purpose: a restart ends the render, so the value dies with it.
// The orchestrator shares one board between the render worker and the API.
type RenderProgressBoard struct {
	mu   sync.Mutex
	jobs map[uuid.UUID]RenderProgress
}

func NewRenderProgressBoard() *RenderProgressBoard {
	return &RenderProgressBoard{jobs: make(map[uuid.UUID]RenderProgress)}
}

func (b *RenderProgressBoard) Set(id uuid.UUID, p RenderProgress) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.jobs[id] = p
}

func (b *RenderProgressBoard) Get(id uuid.UUID) (RenderProgress, bool) {
	if b == nil {
		return RenderProgress{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.jobs[id]
	return p, ok
}

func (b *RenderProgressBoard) Clear(id uuid.UUID) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.jobs, id)
}
