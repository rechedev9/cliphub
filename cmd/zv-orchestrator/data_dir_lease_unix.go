//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func openDataDirLeaseFile(dataDir string) (*os.File, string, error) {
	lockPath := filepath.Join(dataDir, dataDirLeaseFilename)
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, lockPath, err
	}
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		_ = unix.Close(fd)
		return nil, lockPath, err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Nlink != 1 {
		_ = unix.Close(fd)
		return nil, lockPath, syscall.EPERM
	}
	return os.NewFile(uintptr(fd), lockPath), lockPath, nil
}

func lockDataDirFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlockDataDirFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
