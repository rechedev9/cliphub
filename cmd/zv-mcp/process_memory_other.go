//go:build !windows

package main

func limitProcessMemory(uintptr) error {
	return nil
}
