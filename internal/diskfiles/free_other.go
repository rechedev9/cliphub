//go:build !windows

package diskfiles

import "golang.org/x/sys/unix"

// FreeBytes is the space an unprivileged user can still write on the disk of dir.
func FreeBytes(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
