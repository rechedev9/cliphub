//go:build !windows

package recording

import "fmt"

func SettingsOwnerAlive(_ int) (bool, error) {
	return false, fmt.Errorf("settings recovery requires the Windows capture host")
}
