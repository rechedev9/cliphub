package recapplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/storage"
)

func StoreFacts(store storage.Storage, id uuid.UUID, facts Facts) error {
	if facts.SchemaVersion != DocumentVersion {
		return fmt.Errorf("unsupported full demo facts version")
	}
	return putJSON(store, artifacts.FullDemoFactsKey(id), facts)
}

func LoadFacts(store storage.Storage, id uuid.UUID) (Facts, bool, error) {
	var facts Facts
	found, err := readJSON(store, artifacts.FullDemoFactsKey(id), &facts)
	if err == nil && found && facts.SchemaVersion != DocumentVersion {
		err = fmt.Errorf("unsupported full demo facts version")
	}
	return facts, found, err
}

// SaveDocument stores an immutable plan before advancing the draft pointer.
// Identical polls can retain the same content hash while receiving fresh IDs.
func SaveDocument(store storage.Storage, id uuid.UUID, document Document) error {
	if err := document.Validate(); err != nil {
		return err
	}
	planID, err := uuid.Parse(document.PlanID)
	if err != nil {
		return err
	}
	key := artifacts.FullDemoPlanKey(id, planID)
	if exists, err := store.Exists(key); err != nil {
		return err
	} else if exists {
		previous, _, err := LoadDocument(store, id, planID)
		if err != nil {
			return err
		}
		if previous.PlanHash != document.PlanHash {
			return &Error{ErrPlanStale, "plan IDs are immutable"}
		}
	} else if err := putJSON(store, key, document); err != nil {
		return err
	}
	return putJSON(store, artifacts.FullDemoCurrentPlanKey(id), struct {
		PlanID string `json:"plan_id"`
	}{document.PlanID})
}

// LoadDocument reads a stored plan. A plan written before the sponsor became a
// bumper slot reads as absent, so the producer plans again from the defaults.
func LoadDocument(store storage.Storage, id, planID uuid.UUID) (Document, bool, error) {
	b, found, err := readBytes(store, artifacts.FullDemoPlanKey(id, planID))
	if err != nil || !found {
		return Document{}, found, err
	}
	if hasRetiredSponsor(b) {
		return Document{}, false, nil
	}
	var document Document
	if err := decodeStrict(b, &document); err != nil {
		return Document{}, false, fmt.Errorf("read %s: %w", artifacts.FullDemoPlanKey(id, planID), err)
	}
	return document, true, document.Validate()
}

func LoadCurrentDocument(store storage.Storage, id uuid.UUID) (Document, bool, error) {
	var pointer struct {
		PlanID string `json:"plan_id"`
	}
	found, err := readJSON(store, artifacts.FullDemoCurrentPlanKey(id), &pointer)
	if err != nil || !found {
		return Document{}, found, err
	}
	planID, err := uuid.Parse(pointer.PlanID)
	if err != nil {
		return Document{}, false, fmt.Errorf("invalid stored full demo plan pointer: %w", err)
	}
	return LoadDocument(store, id, planID)
}

func putJSON(store storage.Storage, key string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	if err := store.Put(key, bytes.NewReader(b)); err != nil {
		return fmt.Errorf("store %s: %w", key, err)
	}
	return nil
}

func readBytes(store storage.Storage, key string) ([]byte, bool, error) {
	r, err := store.Open(key)
	if err != nil {
		if storage.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, (4<<20)+1))
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", key, err)
	}
	return b, true, nil
}

func readJSON(store storage.Storage, key string, value any) (bool, error) {
	b, found, err := readBytes(store, key)
	if err != nil || !found {
		return found, err
	}
	if err := decodeStrict(b, value); err != nil {
		return false, fmt.Errorf("read %s: %w", key, err)
	}
	return true, nil
}
