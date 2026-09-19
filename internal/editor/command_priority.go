package editor

import (
	"context"
	"os/exec"
)

// Full Demo runs two branches at once and only one of them is the critical
// path. Every process of the audio branch's mastering chain is a single
// threaded loudnorm or AAC pass whose measured uncontended cost over the saved
// 930.6 s program is 12.37 s (sigma 0.04 s over eight probes, unchanged by
// input size, null-sink format or -threads). In the paired render that chain
// carried ~95 s of pure contention tax, all of it inside the window where the
// three wide video item pool and the speculative Media Foundation recovery ran,
// while the video branch itself finished 90 s before mastering did.
//
// Work that has that much slack is marked here so its subprocesses start in a
// lower scheduling class. Scheduling priority is work conserving: the marked
// work still gets the whole machine whenever the critical path is idle, and it
// cannot reach any encoder's output, so no delivered byte depends on it.

type backgroundProcessPriorityKey struct{}

// withBackgroundProcessPriority marks ctx as off the render's critical path.
// Only processes started under the returned context are affected.
func withBackgroundProcessPriority(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundProcessPriorityKey{}, true)
}

func backgroundProcessPriority(ctx context.Context) bool {
	marked, _ := ctx.Value(backgroundProcessPriorityKey{}).(bool)
	return marked
}

// applyBackgroundProcessPriority is called once per subprocess, immediately
// before it is started, from the single execution choke point. An unmarked
// context leaves the command exactly as it was built.
func applyBackgroundProcessPriority(ctx context.Context, cmd *exec.Cmd) {
	if cmd == nil || cmd.Process != nil || !backgroundProcessPriority(ctx) {
		return
	}
	setBackgroundProcessPriority(cmd)
}
