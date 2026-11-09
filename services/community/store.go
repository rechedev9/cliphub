package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// store is the service's SQLite file. The server reads it while the daily
// indexer writes it, so it runs in WAL mode with a busy timeout.
type store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS pro_demos (
	dataset    TEXT NOT NULL,
	path       TEXT NOT NULL,
	sha256     TEXT NOT NULL,
	size       INTEGER NOT NULL,
	event      TEXT NOT NULL,
	status     TEXT NOT NULL,
	attempts   INTEGER NOT NULL DEFAULT 0,
	error      TEXT NOT NULL DEFAULT '',
	summary    TEXT,
	indexed_at TEXT NOT NULL,
	PRIMARY KEY (dataset, path)
);
CREATE TABLE IF NOT EXISTS pro_reference (
	map     TEXT PRIMARY KEY,
	body    BLOB NOT NULL,
	etag    TEXT NOT NULL,
	built_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS ranking_lines (
	install_id TEXT NOT NULL,
	map        TEXT NOT NULL,
	rounds     INTEGER NOT NULL,
	matches    INTEGER NOT NULL,
	vals       TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (install_id, map)
);
CREATE INDEX IF NOT EXISTS ranking_lines_map ON ranking_lines (map);
CREATE TABLE IF NOT EXISTS ranking_submits (
	install_id TEXT NOT NULL,
	day        TEXT NOT NULL,
	count      INTEGER NOT NULL,
	PRIMARY KEY (install_id, day)
);
`

func openStore(path string) (*store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=NORMAL", "PRAGMA busy_timeout=10000", "PRAGMA foreign_keys=ON", schema} {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize database: %w", err)
		}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("restrict database: %w", err)
	}
	return &store{db: db}, nil
}

func (s *store) Close() error { return s.db.Close() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// referenceBody returns the stored payload of one key ("index", "all" or a map).
func (s *store) referenceBody(ctx context.Context, key string) ([]byte, string, error) {
	var body []byte
	var etag string
	err := s.db.QueryRowContext(ctx, `SELECT body, etag FROM pro_reference WHERE map = ?`, key).Scan(&body, &etag)
	return body, etag, err
}

// replaceReference swaps every pro reference payload in one transaction; the
// lineup payloads that share the table are rebuilt on their own.
func (s *store) replaceReference(ctx context.Context, payloads map[string][]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM pro_reference WHERE map NOT LIKE 'lineups:%' AND map NOT LIKE 'pro-players:%'`); err != nil {
		return err
	}
	at := now()
	for key, body := range payloads {
		if _, err := tx.ExecContext(ctx, `INSERT INTO pro_reference (map, body, etag, built_at) VALUES (?, ?, ?, ?)`, key, body, etagOf(body), at); err != nil {
			return err
		}
	}
	return tx.Commit()
}
