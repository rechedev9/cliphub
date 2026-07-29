package anticheat

import (
	"sync"

	"github.com/google/uuid"
)

// JobLocks serializes durable analysis-document transitions for one demo while
// allowing independent demos to enter the screening lane concurrently. Entries
// are removed after their final waiter releases them so a long-running Studio
// does not retain every completed job ID.
type JobLocks struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*jobLock
}

type jobLock struct {
	mu   sync.Mutex
	refs int
}

// NewJobLocks returns an empty per-job lock coordinator.
func NewJobLocks() *JobLocks {
	return &JobLocks{locks: make(map[uuid.UUID]*jobLock)}
}

// Lock acquires id's lock and returns an idempotent release function.
func (l *JobLocks) Lock(id uuid.UUID) func() {
	if l == nil {
		return func() {}
	}
	l.mu.Lock()
	entry := l.locks[id]
	if entry == nil {
		entry = &jobLock{}
		l.locks[id] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()
	var once sync.Once
	return func() {
		once.Do(func() {
			entry.mu.Unlock()
			l.mu.Lock()
			entry.refs--
			if entry.refs == 0 {
				delete(l.locks, id)
			}
			l.mu.Unlock()
		})
	}
}
