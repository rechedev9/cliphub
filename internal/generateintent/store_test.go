package generateintent

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func (s *Store) Write(id uuid.UUID, intent renderplan.GenerateIntent) error {
	lock := s.lock(id)
	lock.Lock()
	defer lock.Unlock()
	return s.write(id, intent)
}

type blockingStorage struct {
	mu           sync.Mutex
	files        map[string][]byte
	blockClose   bool
	readStarted  chan struct{}
	releaseClose chan struct{}
}

func (s *blockingStorage) Put(key string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.files[key] = b
	s.mu.Unlock()
	return nil
}

func (s *blockingStorage) Open(key string) (io.ReadCloser, error) {
	s.mu.Lock()
	b, ok := s.files[key]
	block := s.blockClose
	if block {
		s.blockClose = false
	}
	s.mu.Unlock()
	if !ok {
		return nil, os.ErrNotExist
	}
	if !block {
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	close(s.readStarted)
	return &blockingReadCloser{Reader: bytes.NewReader(b), release: s.releaseClose}, nil
}

func (s *blockingStorage) Exists(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.files[key]
	return ok, nil
}

type blockingReadCloser struct {
	*bytes.Reader
	release <-chan struct{}
}

func (r *blockingReadCloser) Close() error {
	<-r.release
	return nil
}

func TestCompleteCannotEraseNewerAcceptedRun(t *testing.T) {
	id := uuid.New()
	oldIntent := renderplan.GenerateIntent{
		Variant:     editor.PresetViral60Clean,
		Edit:        renderplan.DefaultEditRequest(),
		ActiveRunID: uuid.New(),
	}
	newIntent := oldIntent
	newIntent.ActiveRunID = uuid.New()
	initial, err := json.Marshal(oldIntent)
	if err != nil {
		t.Fatal(err)
	}
	backend := &blockingStorage{
		files:        map[string][]byte{artifacts.GenerateIntentKey(id): initial},
		blockClose:   true,
		readStarted:  make(chan struct{}),
		releaseClose: make(chan struct{}),
	}
	store := New(backend)

	completeDone := make(chan error, 1)
	go func() { completeDone <- store.Complete(id, oldIntent.ActiveRunID) }()
	<-backend.readStarted

	writeDone := make(chan error, 1)
	writeStarted := make(chan struct{})
	go func() {
		close(writeStarted)
		writeDone <- store.Write(id, newIntent)
	}()
	<-writeStarted
	var earlyWriteErr error
	writeFinishedEarly := false
	select {
	case earlyWriteErr = <-writeDone:
		writeFinishedEarly = true
	case <-time.After(100 * time.Millisecond):
	}

	close(backend.releaseClose)
	if err := <-completeDone; err != nil {
		t.Fatalf("Complete error = %v", err)
	}
	if writeFinishedEarly {
		t.Fatalf("new run write completed before old compare-and-write: %v", earlyWriteErr)
	}
	if writeErr := <-writeDone; writeErr != nil {
		t.Fatalf("Write error = %v", writeErr)
	}
	got, ok, err := store.Read(id)
	if err != nil || !ok {
		t.Fatalf("Read = (%#v, %v, %v)", got, ok, err)
	}
	if got.ActiveRunID != newIntent.ActiveRunID {
		t.Fatalf("ActiveRunID = %s, want newer run %s", got.ActiveRunID, newIntent.ActiveRunID)
	}
}

func TestBeginRejectsOverlappingRunBeforeEligibilityCheck(t *testing.T) {
	id := uuid.New()
	active := renderplan.GenerateIntent{
		Variant:     editor.PresetViral60Clean,
		Edit:        renderplan.DefaultEditRequest(),
		ActiveRunID: uuid.New(),
	}
	backend := &blockingStorage{files: make(map[string][]byte)}
	store := New(backend)
	if err := store.Write(id, active); err != nil {
		t.Fatal(err)
	}
	eligibilityCalled := false
	next := active
	next.ActiveRunID = uuid.New()
	err := store.Begin(id, next, func() error {
		eligibilityCalled = true
		return nil
	})
	if !errors.Is(err, ErrActiveRun) {
		t.Fatalf("Begin error = %v, want ErrActiveRun", err)
	}
	if eligibilityCalled {
		t.Fatal("eligibility ran while an active run already owned the job")
	}
}

func TestFinishDoesNotPublishForStaleRun(t *testing.T) {
	id := uuid.New()
	current := renderplan.GenerateIntent{
		Variant:     editor.PresetViral60Clean,
		Edit:        renderplan.DefaultEditRequest(),
		ActiveRunID: uuid.New(),
	}
	backend := &blockingStorage{files: make(map[string][]byte)}
	store := New(backend)
	if err := store.Write(id, current); err != nil {
		t.Fatal(err)
	}
	published := false
	owned, err := store.Finish(id, uuid.New(), func() error {
		published = true
		return nil
	})
	if err != nil {
		t.Fatalf("Finish error = %v", err)
	}
	if owned || published {
		t.Fatalf("Finish = owned %v published %v, want false/false", owned, published)
	}
	got, ok, err := store.Read(id)
	if err != nil || !ok {
		t.Fatalf("Read = (%#v, %v, %v)", got, ok, err)
	}
	if got.ActiveRunID != current.ActiveRunID {
		t.Fatalf("ActiveRunID = %s, want %s", got.ActiveRunID, current.ActiveRunID)
	}
}

func TestFailKeepsItsCauseUntilTheNextAcceptedRun(t *testing.T) {
	id := uuid.New()
	store := New(&blockingStorage{files: map[string][]byte{}})
	run := renderplan.GenerateIntent{Variant: editor.PresetViral60Clean, Edit: renderplan.DefaultEditRequest(), ActiveRunID: uuid.New()}
	if err := store.Begin(id, run, nil); err != nil {
		t.Fatal(err)
	}
	if owned, err := store.Fail(id, uuid.New(), errors.New("a superseded run"), nil); owned || err != nil {
		t.Fatalf("Fail of a run that does not own the job = (%v, %v), want it ignored", owned, err)
	}
	if got, _, err := store.Read(id); err != nil || got.ActiveRunID != run.ActiveRunID || got.FailureReason != "" {
		t.Fatalf("intent after a superseded run failed = %+v (%v), want the active run untouched", got, err)
	}
	if owned, err := store.Fail(id, run.ActiveRunID, errors.New("recorder crashed"), nil); !owned || err != nil {
		t.Fatalf("Fail = (%v, %v), want the owning run closed", owned, err)
	}
	failed, _, err := store.Read(id)
	if err != nil || failed.ActiveRunID != uuid.Nil || failed.FailureReason != "recorder crashed" || failed.FailedAt.IsZero() {
		t.Fatalf("intent after its run failed = %+v (%v), want it idle with the cause and its time", failed, err)
	}
	next := run
	next.ActiveRunID = uuid.New()
	if err := store.Begin(id, next, nil); err != nil {
		t.Fatal(err)
	}
	if got, _, err := store.Read(id); err != nil || got.FailureReason != "" || !got.FailedAt.IsZero() {
		t.Fatalf("intent of the next accepted run = %+v (%v), want no earlier failure", got, err)
	}
	if _, err := store.Fail(id, next.ActiveRunID, errors.New("recorder crashed again"), nil); err != nil {
		t.Fatal(err)
	}
	if err := store.ClearFailure(id); err != nil {
		t.Fatal(err)
	}
	if got, _, err := store.Read(id); err != nil || got.FailureReason != "" || !got.FailedAt.IsZero() || got.Variant != next.Variant {
		t.Fatalf("intent after ClearFailure = %+v (%v), want the same choice without the failure", got, err)
	}
	if err := store.ClearFailure(uuid.New()); err != nil {
		t.Fatalf("ClearFailure of a job with no intent = %v, want nothing to do", err)
	}
}
