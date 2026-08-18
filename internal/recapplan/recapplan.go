// Package recapplan loads and stores the sidecar full-round plan used when
// match_recap is on. The job kill plan stays the Shorts burst windows.
package recapplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func Load(store storage.Storage, id uuid.UUID) (killplan.Plan, bool, error) {
	rc, err := store.Open(artifacts.RecapPlanKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return killplan.Plan{}, false, nil
		}
		return killplan.Plan{}, false, fmt.Errorf("open recap plan: %w", err)
	}
	defer rc.Close()
	plan, err := decode(rc)
	if err != nil {
		return killplan.Plan{}, false, err
	}
	return plan, true, nil
}

func Store(store storage.Storage, id uuid.UUID, plan killplan.Plan) error {
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := store.Put(artifacts.RecapPlanKey(id), bytes.NewReader(b)); err != nil {
		return fmt.Errorf("store recap plan: %w", err)
	}
	return nil
}

func decode(r io.Reader) (killplan.Plan, error) {
	var plan killplan.Plan
	if err := json.NewDecoder(r).Decode(&plan); err != nil {
		return killplan.Plan{}, fmt.Errorf("decode recap plan: %w", err)
	}
	return plan, nil
}
