package recapplan

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
)

type memStore struct {
	files map[string][]byte
}

func (s *memStore) Put(key string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[key] = b
	return nil
}

func (s *memStore) Open(key string) (io.ReadCloser, error) {
	b, ok := s.files[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (s *memStore) Exists(key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func TestLoadStoreRoundtrip(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	store := &memStore{}
	if _, ok, err := Load(store, id); err != nil || ok {
		t.Fatalf("Load missing = (%v, %v), want (false, nil)", ok, err)
	}

	want := killplan.NewPlan()
	want.Segments = []killplan.Segment{{
		ID:        "seg-001",
		Round:     3,
		TickStart: 1000,
		TickEnd:   8000,
	}}
	if err := Store(store, id, want); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.files[artifacts.RecapPlanKey(id)]; !ok {
		t.Fatal("Store did not write RecapPlanKey")
	}

	got, ok, err := Load(store, id)
	if err != nil || !ok {
		t.Fatalf("Load = (%v, %v), want plan", ok, err)
	}
	if len(got.Segments) != 1 || got.Segments[0].Round != 3 || got.Segments[0].TickStart != 1000 {
		t.Fatalf("recap segments = %#v, want the stored round window", got.Segments)
	}
}
