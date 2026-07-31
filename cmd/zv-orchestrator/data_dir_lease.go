package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const dataDirLeaseFilename = ".tickcut.lock"

// dataDirLease holds an OS-backed exclusive lock for every mutable artifact
// rooted at one ZV_DATA_DIR. The kernel releases it on process death, unlike a
// sentinel file, so a crashed Studio cannot permanently strand the data root.
type dataDirLease struct {
	file *os.File
}

type dataDirLeaseOwner struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func acquireDataDirLease(dataDir string) (*dataDirLease, error) {
	absDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	file, lockPath, err := openDataDirLeaseFile(absDir)
	if err != nil {
		return nil, fmt.Errorf("open data directory lease %s: %w", lockPath, err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("restrict data directory lease: %w", err)
	}
	if err := lockDataDirFile(file); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf(
			"data directory %s is already owned by another TickCut process: %w",
			absDir,
			err,
		)
	}

	owner, err := json.Marshal(dataDirLeaseOwner{PID: os.Getpid(), StartedAt: time.Now().UTC()})
	if err != nil {
		_ = unlockDataDirFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("encode data directory lease owner: %w", err)
	}
	if err := file.Truncate(0); err != nil {
		_ = unlockDataDirFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("reset data directory lease owner: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = unlockDataDirFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("seek data directory lease owner: %w", err)
	}
	if _, err := file.Write(append(owner, '\n')); err != nil {
		_ = unlockDataDirFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("write data directory lease owner: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = unlockDataDirFile(file)
		_ = file.Close()
		return nil, fmt.Errorf("sync data directory lease owner: %w", err)
	}
	return &dataDirLease{file: file}, nil
}

func (l *dataDirLease) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unlockDataDirFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("unlock data directory lease: %w", unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close data directory lease: %w", closeErr)
	}
	return nil
}
