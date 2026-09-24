package telemetry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// BackupCopies is how many daily copies of each database are kept.
const BackupCopies = 7

var backupFilePattern = regexp.MustCompile(`^(events|logs)-[0-9]{8}\.db$`)

// BackupDaily writes today's VACUUM INTO copy of the events and logs databases
// into dir, then keeps only the newest BackupCopies files of each. A copy that
// already exists for the day is kept, so a restart does not rewrite it. The
// caller runs it after retention, so copies never hold expired rows.
func (s *Store) BackupDaily(ctx context.Context, dir string, now time.Time) (int, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, fmt.Errorf("create telemetry backup directory: %w", err)
	}
	// #nosec G302 -- directories require execute permission; 0700 is owner-only.
	if err := os.Chmod(dir, 0o700); err != nil {
		return 0, fmt.Errorf("restrict telemetry backup directory: %w", err)
	}
	written := 0
	for _, target := range []struct {
		name string
		db   *sql.DB
	}{{"events", s.db}, {"logs", s.logs.db}} {
		wrote, err := backupDatabase(ctx, target.db, dir, target.name, now)
		if err != nil {
			return written, err
		}
		if wrote {
			written++
		}
		if err := pruneBackups(dir, target.name, BackupCopies); err != nil {
			return written, err
		}
	}
	return written, nil
}

func backupDatabase(ctx context.Context, db *sql.DB, dir, name string, now time.Time) (bool, error) {
	final := filepath.Join(dir, name+"-"+now.UTC().Format("20060102")+".db")
	if _, err := os.Stat(final); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect %s backup: %w", name, err)
	}
	// VACUUM INTO refuses an existing target, and an interrupted copy must not
	// pass for a complete one: write a temporary file and rename it at the end.
	temporary := final + ".tmp"
	if err := os.Remove(temporary); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("remove stale %s backup: %w", name, err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", temporary); err != nil {
		_ = os.Remove(temporary)
		return false, fmt.Errorf("copy %s database: %w", name, err)
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return false, fmt.Errorf("restrict %s backup: %w", name, err)
	}
	if err := os.Rename(temporary, final); err != nil {
		_ = os.Remove(temporary)
		return false, fmt.Errorf("publish %s backup: %w", name, err)
	}
	return true, nil
}

func pruneBackups(dir, name string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list telemetry backups: %w", err)
	}
	var copies []string
	for _, entry := range entries {
		match := backupFilePattern.FindStringSubmatch(entry.Name())
		if entry.Type().IsRegular() && match != nil && match[1] == name {
			copies = append(copies, entry.Name())
		}
	}
	// YYYYMMDD names sort chronologically.
	sort.Sort(sort.Reverse(sort.StringSlice(copies)))
	for _, old := range copies[min(keep, len(copies)):] {
		if err := os.Remove(filepath.Join(dir, old)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("prune telemetry backup: %w", err)
		}
	}
	return nil
}
