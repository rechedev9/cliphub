package jobprogress

import (
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
)

type memStore struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (s *memStore) Put(key string, r io.Reader) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[key] = body
	return nil
}

func (s *memStore) Open(key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, ok := s.files[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytesReader(body)), nil
}

func (s *memStore) Exists(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.files[key]
	return ok, nil
}

func bytesReader(b []byte) *bytesBuf { return &bytesBuf{b: b} }

type bytesBuf struct {
	b []byte
	i int
}

func (r *bytesBuf) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

func TestReporterWritesFirstAndComplete(t *testing.T) {
	store := &memStore{}
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rep := NewReporter(store, id, StageParse, UnitTicks, "ticks")
	n := 0
	rep.now = func() time.Time {
		n++
		return time.Unix(10, 0).UTC().Add(time.Duration(n) * DefaultMinInterval)
	}

	if err := rep.Update(0, 100); err != nil {
		t.Fatal(err)
	}
	snap, ok, err := LoadJob(store, id)
	if err != nil || !ok {
		t.Fatalf("load after first write: ok=%v err=%v", ok, err)
	}
	if snap.Done != 0 || snap.Total != 100 || snap.Percent != 0 {
		t.Fatalf("first snapshot = %+v", snap)
	}
	if snap.Stage != StageParse || artifacts.ProgressKey(id) == "" {
		t.Fatalf("stage = %q", snap.Stage)
	}

	if err := rep.Update(40, 100); err != nil {
		t.Fatal(err)
	}
	// Throttle swallows a same-percent rewrite, but 40% is a new percent so it lands.
	snap, ok, err = LoadJob(store, id)
	if err != nil || !ok || snap.Percent != 40 {
		t.Fatalf("mid snapshot = %+v ok=%v err=%v", snap, ok, err)
	}

	if err := rep.Complete(100); err != nil {
		t.Fatal(err)
	}
	snap, ok, err = LoadJob(store, id)
	if err != nil || !ok || snap.Percent != 100 || snap.Done != 100 {
		t.Fatalf("complete snapshot = %+v ok=%v err=%v", snap, ok, err)
	}
}

func TestReporterThrottlesUnchangedPercent(t *testing.T) {
	store := &memStore{}
	id := uuid.New()
	rep := NewReporter(store, id, StageScan, UnitTicks, "ticks")
	rep.minGap = time.Hour
	n := 0
	base := time.Unix(20, 0).UTC()
	rep.now = func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Millisecond)
	}
	if err := rep.Update(10, 100); err != nil {
		t.Fatal(err)
	}
	first := len(store.files[artifacts.ProgressKey(id)])
	if err := rep.Update(10, 100); err != nil {
		t.Fatal(err)
	}
	if len(store.files[artifacts.ProgressKey(id)]) != first {
		t.Fatal("throttled rewrite still replaced the snapshot")
	}
}

func TestReporterThrottlesDoneChangesWithinGap(t *testing.T) {
	store := &memStore{}
	id := uuid.New()
	rep := NewReporter(store, id, StageParse, UnitTicks, "ticks")
	rep.minGap = time.Hour
	n := 0
	base := time.Unix(30, 0).UTC()
	rep.now = func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Millisecond)
	}
	if err := rep.Update(1, 100); err != nil {
		t.Fatal(err)
	}
	first := string(store.files[artifacts.ProgressKey(id)])
	if err := rep.Update(2, 100); err != nil {
		t.Fatal(err)
	}
	if string(store.files[artifacts.ProgressKey(id)]) != first {
		t.Fatal("tick walk within the gap rewrote progress.json")
	}
}

func TestLoadMissingIsNotAnError(t *testing.T) {
	_, ok, err := LoadJob(&missingStore{}, uuid.New())
	if err != nil || ok {
		t.Fatalf("missing load ok=%v err=%v", ok, err)
	}
}

type missingStore struct{}

func (missingStore) Put(string, io.Reader) error { return nil }
func (missingStore) Open(string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}
func (missingStore) Exists(string) (bool, error) { return false, nil }
