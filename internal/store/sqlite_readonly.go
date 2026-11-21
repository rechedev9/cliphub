package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var sqliteURIPathEscapes = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

func OpenSQLiteLibraryReadOnly(path string) (*Repositories, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("open sqlite read-only: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+sqliteURIPathEscapes.Replace(filepath.ToSlash(path))+"?"+strings.Join([]string{
		"mode=ro",
		"_pragma=busy_timeout(5000)",
		"_pragma=query_only(1)",
	}, "&"))
	if err != nil {
		return nil, fmt.Errorf("open sqlite read-only: %w", err)
	}
	version, err := schemaVersion(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if version != len(migrations) {
		_ = db.Close()
		return nil, fmt.Errorf("jobs.db schema version %d is not the one this build reads (%d); open the library with this build's Studio first", version, len(migrations))
	}
	return &Repositories{Library: NewSQLiteLibraryRepository(db), Streams: &SQLiteStreamJobRepository{db: db}, db: db}, nil
}
