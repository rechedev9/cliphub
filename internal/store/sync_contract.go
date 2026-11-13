package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// A device sends its SyncContract in these headers; the hub refuses a different
// one, so no side ever applies a row its schema cannot hold.
const (
	SyncContractHeader = "X-ClipMind-Sync-Contract"
	SyncSchemaHeader   = "X-ClipMind-Sync-Schema"
)

// The codes a device reads to say who has to update.
const (
	CodeSyncClientOutdated = "sync_client_outdated"
	CodeSyncServerOutdated = "sync_server_outdated"
)

// SyncContract is what two jobs.db must share to exchange changes: the columns
// of every synced table. Schema is the build's migration count, which says the older side.
type SyncContract struct {
	Hash   string
	Schema int
}

// Contract reads the synced tables once: an open database's schema does not change.
func (s *SyncLog) Contract(ctx context.Context) (SyncContract, error) {
	s.contractOnce.Do(func() { s.contract, s.contractErr = readSyncContract(ctx, s.db) })
	return s.contract, s.contractErr
}

func readSyncContract(ctx context.Context, db *sql.DB) (SyncContract, error) {
	if db == nil {
		return SyncContract{}, errors.New("sync contract: no database")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SyncContract{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var b strings.Builder
	for _, table := range syncTables {
		columns, err := describeSyncTable(ctx, tx, table)
		if err != nil {
			return SyncContract{}, fmt.Errorf("sync contract %s: %w", table.name, err)
		}
		names := make([]string, 0, len(columns))
		for name := range columns {
			names = append(names, name)
		}
		sort.Strings(names)
		b.WriteString(table.name)
		for _, name := range names {
			c := columns[name]
			fmt.Fprintf(&b, " %s:%s:%t", name, strings.ToUpper(c.declared), c.notNull)
		}
		b.WriteString(";")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return SyncContract{Hash: hex.EncodeToString(sum[:8]), Schema: len(migrations)}, nil
}
