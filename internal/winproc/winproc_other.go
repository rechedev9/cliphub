//go:build !windows

package winproc

// Running always answers no off Windows: capture, Steam and CS2 are Windows only.
func Running(string) (bool, error) { return false, nil }

// WindowTitles always answers no off Windows.
func WindowTitles(string) (bool, []string, error) { return false, nil, nil }
