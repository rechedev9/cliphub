package diskfiles

import "golang.org/x/sys/windows"

// FreeBytes is the space the current user can still write on the disk of dir.
func FreeBytes(dir string) (uint64, error) {
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free uint64
	err = windows.GetDiskFreeSpaceEx(path, &free, nil, nil)
	return free, err
}
