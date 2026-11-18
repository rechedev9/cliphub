package store

import (
	"context"
	"testing"
)

// A Studio from before the demo title (migration 30) has another contract, so the hub answers it 409.
func TestSyncContractMatchesSameSchemaAndChangesWithASyncedColumn(t *testing.T) {
	ctx := context.Background()
	_, a := newTestSyncLog(t)
	dbB, b := newTestSyncLog(t)
	ca, err := a.Contract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dbB.Exec(`ALTER TABLE library_demos DROP COLUMN title`); err != nil {
		t.Fatal(err)
	}
	cb, err := b.Contract(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ca.Schema != len(migrations) || ca.Hash == "" {
		t.Fatalf("contract = %+v", ca)
	}
	if ca.Hash == cb.Hash {
		t.Fatal("a database without the demo title kept the same contract")
	}
	_, c := newTestSyncLog(t)
	if cc, _ := c.Contract(ctx); cc != ca {
		t.Fatalf("same schema, different contract: %+v vs %+v", cc, ca)
	}
}
