package argv

import (
	"context"
	"os/exec"
)

func Command(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...) // #nosec G204
}

func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...) // #nosec G204
}
