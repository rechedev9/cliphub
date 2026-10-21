// Package winproc reads the process list and window titles in-process, so a
// check starts no child process next to a running CS2.
package winproc

// Process is one entry of the system process list.
type Process struct {
	PID       uint32
	ParentPID uint32
	Exe       string
}
