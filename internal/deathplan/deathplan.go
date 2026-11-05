// Package deathplan loads and stores the sidecar plan of the target's deaths:
// one clip per death, from a few seconds before it to just after it.
package deathplan

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

// Load returns the job's death plan; found is false for a job parsed before
// death clips existed.
func Load(store storage.Storage, id uuid.UUID) (plan killplan.Plan, found bool, err error) {
	rc, err := store.Open(artifacts.DeathPlanKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return killplan.Plan{}, false, nil
		}
		return killplan.Plan{}, false, fmt.Errorf("open death plan: %w", err)
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(&plan); err != nil {
		return killplan.Plan{}, false, fmt.Errorf("decode death plan: %w", err)
	}
	return plan, true, nil
}

func Store(store storage.Storage, id uuid.UUID, plan killplan.Plan) error {
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := store.Put(artifacts.DeathPlanKey(id), bytes.NewReader(b)); err != nil {
		return fmt.Errorf("store death plan: %w", err)
	}
	return nil
}
