package jobprogress

import (
	"bytes"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/storage"
)

// DefaultMinInterval bounds how often a reporter rewrites the snapshot.
// Tick walks fire ~64-128 times a second; the UI polls every 0.8-3s, so
// sub-200ms writes only fight the disk.
const DefaultMinInterval = 200 * time.Millisecond

// Reporter writes a throttled durable snapshot. First and last updates always
// land so a poll that arrives mid-stage still sees a real total.
type Reporter struct {
	store    storage.Storage
	key      string
	stage    string
	unit     string
	label    string
	minGap   time.Duration
	now      func() time.Time
	mu       sync.Mutex
	lastAt   time.Time
	lastPct  int
	lastDone int64
	wrote    bool
}

func NewReporter(store storage.Storage, id uuid.UUID, stage, unit, label string) *Reporter {
	return &Reporter{
		store:  store,
		key:    artifacts.ProgressKey(id),
		stage:  stage,
		unit:   unit,
		label:  label,
		minGap: DefaultMinInterval,
		now:    time.Now,
	}
}

func NewKeyedReporter(store storage.Storage, key, stage, unit, label string) *Reporter {
	return &Reporter{
		store:  store,
		key:    key,
		stage:  stage,
		unit:   unit,
		label:  label,
		minGap: DefaultMinInterval,
		now:    time.Now,
	}
}

// Update persists done/total when the percent moved, the interval elapsed, or
// this is the first write (so the UI can show 0 / N immediately).
func (r *Reporter) Update(done, total int64) error {
	if r == nil {
		return nil
	}
	snap, err := NewSnapshot(r.stage, r.unit, r.label, done, total, r.clock())
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.wrote && r.minGap > 0 && !r.lastAt.IsZero() && snap.UpdatedAt.Sub(r.lastAt) < r.minGap {
		return nil
	}
	if err := r.put(snap); err != nil {
		return err
	}
	r.lastAt = snap.UpdatedAt
	r.lastPct = snap.Percent
	r.lastDone = snap.Done
	r.wrote = true
	return nil
}

// Complete writes the final 100% snapshot, ignoring the throttle.
func (r *Reporter) Complete(total int64) error {
	if r == nil {
		return nil
	}
	if total < 0 {
		total = 0
	}
	return r.force(total, total)
}

func (r *Reporter) force(done, total int64) error {
	snap, err := NewSnapshot(r.stage, r.unit, r.label, done, total, r.clock())
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.put(snap); err != nil {
		return err
	}
	r.lastAt = snap.UpdatedAt
	r.lastPct = snap.Percent
	r.lastDone = snap.Done
	r.wrote = true
	return nil
}

func (r *Reporter) put(snap Snapshot) error {
	var buf bytes.Buffer
	if err := snap.Encode(&buf); err != nil {
		return err
	}
	return r.store.Put(r.key, bytes.NewReader(buf.Bytes()))
}

func (r *Reporter) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// Load reads a stored snapshot. Missing keys are not an error: ok is false.
func Load(store storage.Storage, key string) (Snapshot, bool, error) {
	rc, err := store.Open(key)
	if err != nil {
		if storage.IsNotExist(err) {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, err
	}
	defer rc.Close()
	snap, err := Decode(rc)
	if err != nil {
		return Snapshot{}, false, err
	}
	return snap, true, nil
}

func LoadJob(store storage.Storage, id uuid.UUID) (Snapshot, bool, error) {
	return Load(store, artifacts.ProgressKey(id))
}
