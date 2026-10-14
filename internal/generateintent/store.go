// Package generateintent owns the durable, job-scoped state for guided
// generate runs.
package generateintent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

const lockStripeCount = 64

// ErrActiveRun reports that a job already owns guided-generate work.
var ErrActiveRun = errors.New("generate run is already active")

var errReentrant = errors.New("generate intent store re-entered under the job lock")

type jobSlot struct {
	mu    sync.Mutex
	owner atomic.Uint64
}

// Store serializes read-modify-write operations for a job's generate intent.
// A single Store is shared by the desktop HTTP and worker runtimes, making a
// conditional completion atomic with accepting a newer run.
type Store struct {
	storage storage.Storage
	slots   [lockStripeCount]jobSlot
}

// New returns a generate-intent store backed by artifacts.
func New(store storage.Storage) *Store {
	return &Store{storage: store}
}

// Begin publishes intent only while no prior run owns the job and ready still
// reports that adjacent render state is idle.
func (s *Store) Begin(id uuid.UUID, intent renderplan.GenerateIntent, ready func() error) error {
	return s.withJob(id, func() error {
		return s.begin(id, intent, ready)
	})
}

func (s *Store) begin(id uuid.UUID, intent renderplan.GenerateIntent, ready func() error) error {
	current, ok, err := s.read(id)
	if err != nil {
		return err
	}
	if ok && current.ActiveRunID != uuid.Nil {
		return fmt.Errorf("%w for job %s", ErrActiveRun, id)
	}
	if ready != nil {
		if err := ready(); err != nil {
			return err
		}
	}
	return s.write(id, intent)
}

// Read returns the latest accepted intent for id, when present.
func (s *Store) Read(id uuid.UUID) (renderplan.GenerateIntent, bool, error) {
	var (
		intent renderplan.GenerateIntent
		ok     bool
		err    error
	)
	lockErr := s.withJob(id, func() error {
		intent, ok, err = s.read(id)
		return nil
	})
	if lockErr != nil {
		return renderplan.GenerateIntent{}, false, lockErr
	}
	return intent, ok, err
}

// Complete clears ActiveRunID only when runID still owns the latest intent.
// The comparison and write share the same critical section as Write, so an
// older run can never erase a newer accepted run.
func (s *Store) Complete(id, runID uuid.UUID) error {
	_, err := s.Finish(id, runID, nil)
	return err
}

// Finish runs persist and clears ActiveRunID only when runID still owns the
// latest intent.
func (s *Store) Finish(id, runID uuid.UUID, persist func() error) (bool, error) {
	if runID == uuid.Nil {
		return false, nil
	}
	var (
		owned bool
		err   error
	)
	lockErr := s.withJob(id, func() error {
		owned, err = s.finish(id, runID, persist)
		return nil
	})
	if lockErr != nil {
		return false, lockErr
	}
	return owned, err
}

func (s *Store) finish(id, runID uuid.UUID, persist func() error) (bool, error) {
	intent, ok, err := s.read(id)
	if err != nil || !ok || intent.ActiveRunID != runID {
		return false, err
	}
	if persist != nil {
		if err := persist(); err != nil {
			return true, err
		}
	}
	intent.ActiveRunID = uuid.Nil
	return true, s.write(id, intent)
}

// WhileIdle runs persist only when no guided run currently owns id.
func (s *Store) WhileIdle(id uuid.UUID, persist func() error) error {
	return s.withJob(id, func() error {
		return s.whileIdle(id, persist)
	})
}

func (s *Store) whileIdle(id uuid.UUID, persist func() error) error {
	intent, ok, err := s.read(id)
	if err != nil {
		return err
	}
	if ok && intent.ActiveRunID != uuid.Nil {
		return fmt.Errorf("%w for job %s", ErrActiveRun, id)
	}
	return persist()
}

func (s *Store) withJob(id uuid.UUID, fn func() error) error {
	slot := &s.slots[int(id[0])%len(s.slots)]
	gid := currentGoroutineID()
	if slot.owner.Load() == gid {
		return errReentrant
	}
	slot.mu.Lock()
	slot.owner.Store(gid)
	defer func() {
		slot.owner.Store(0)
		slot.mu.Unlock()
	}()
	return fn()
}

func currentGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	text := string(buf[:n])
	text = strings.TrimPrefix(text, "goroutine ")
	id, _, _ := strings.Cut(text, " ")
	parsed, _ := strconv.ParseUint(id, 10, 64)
	return parsed
}

func (s *Store) write(id uuid.UUID, intent renderplan.GenerateIntent) error {
	b, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal generate intent: %w", err)
	}
	if err := s.storage.Put(artifacts.GenerateIntentKey(id), bytes.NewReader(b)); err != nil {
		return fmt.Errorf("write generate intent: %w", err)
	}
	return nil
}

func (s *Store) read(id uuid.UUID) (renderplan.GenerateIntent, bool, error) {
	rc, err := s.storage.Open(artifacts.GenerateIntentKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return renderplan.GenerateIntent{}, false, nil
		}
		return renderplan.GenerateIntent{}, false, fmt.Errorf("open generate intent: %w", err)
	}
	var intent renderplan.GenerateIntent
	if err := json.NewDecoder(rc).Decode(&intent); err != nil {
		return renderplan.GenerateIntent{}, false, fmt.Errorf("decode generate intent: %w", errors.Join(err, rc.Close()))
	}
	if err := rc.Close(); err != nil {
		return renderplan.GenerateIntent{}, false, fmt.Errorf("close generate intent: %w", err)
	}
	return intent, true, nil
}
