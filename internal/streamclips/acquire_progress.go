package streamclips

import (
	"sync"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/vodfetch"
)

// AcquireProgress holds the live download progress of acquiring jobs. It is
// in memory on purpose: a restart ends the download, so the value dies with it.
// The orchestrator shares one instance between the acquire worker and the API.
type AcquireProgress struct {
	mu   sync.Mutex
	jobs map[uuid.UUID]vodfetch.Progress
}

func NewAcquireProgress() *AcquireProgress {
	return &AcquireProgress{jobs: make(map[uuid.UUID]vodfetch.Progress)}
}

func (a *AcquireProgress) Set(id uuid.UUID, p vodfetch.Progress) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.jobs[id] = p
}

func (a *AcquireProgress) Get(id uuid.UUID) (vodfetch.Progress, bool) {
	if a == nil {
		return vodfetch.Progress{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.jobs[id]
	return p, ok
}

func (a *AcquireProgress) Clear(id uuid.UUID) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.jobs, id)
}
