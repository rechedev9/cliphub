package workers

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/artifacts"
	"github.com/rechedev9/fragforge/internal/editor"
	"github.com/rechedev9/fragforge/internal/renderplan"
	"github.com/rechedev9/fragforge/internal/storage"
)

type renderRevisionDeleteSpy struct {
	storage.Storage
	deleted []string
}

func (s *renderRevisionDeleteSpy) DeleteTree(prefix string) error {
	s.deleted = append(s.deleted, prefix)
	return nil
}

func TestDeleteRenderVariantRevisionDeletesOnlyExactOwnedRevision(t *testing.T) {
	jobID := uuid.New()
	variant := editor.PresetViral60Clean
	prefix, err := renderplan.RenderVariantRevisionPrefix(jobID, variant, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	store := &renderRevisionDeleteSpy{Storage: newFakeStorage()}

	if err := deleteRenderVariantRevision(store, jobID, variant, prefix); err != nil {
		t.Fatalf("deleteRenderVariantRevision error = %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != prefix {
		t.Fatalf("deleted prefixes = %v, want [%s]", store.deleted, prefix)
	}
}

func TestDeleteRenderVariantRevisionRejectsForeignAndUnsafeNamespaces(t *testing.T) {
	jobID := uuid.New()
	variant := editor.PresetViral60Clean
	revisionID := uuid.New()
	validPrefix, err := renderplan.RenderVariantRevisionPrefix(jobID, variant, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	otherJobPrefix, err := renderplan.RenderVariantRevisionPrefix(uuid.New(), variant, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	otherVariantPrefix, err := renderplan.RenderVariantRevisionPrefix(jobID, "other-variant", revisionID)
	if err != nil {
		t.Fatal(err)
	}
	canonicalPrefix, err := artifacts.RenderVariantPrefix(jobID, variant)
	if err != nil {
		t.Fatal(err)
	}

	for name, prefix := range map[string]string{
		"other job":     otherJobPrefix,
		"other variant": otherVariantPrefix,
		"canonical":     canonicalPrefix,
		"traversal":     validPrefix + "/../" + uuid.NewString(),
	} {
		t.Run(name, func(t *testing.T) {
			store := &renderRevisionDeleteSpy{Storage: newFakeStorage()}
			if err := deleteRenderVariantRevision(store, jobID, variant, prefix); err == nil {
				t.Fatalf("deleteRenderVariantRevision(%q) error = nil, want rejection", prefix)
			}
			if len(store.deleted) != 0 {
				t.Fatalf("deleted prefixes = %v, want none", store.deleted)
			}
		})
	}
}
