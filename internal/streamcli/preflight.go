package streamcli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rechedev9/fragforge/internal/pathguard"
)

type streamInputPath struct {
	flag string
	path string
}

func rejectStreamOutputAliases(output string, inputs ...streamInputPath) error {
	guardInputs := make([]pathguard.Input, len(inputs))
	for i, input := range inputs {
		guardInputs[i] = pathguard.Input{Flag: input.flag, Path: input.path}
	}
	return pathguard.RejectOutputAliases(output, guardInputs...)
}

func rejectStreamInputsWithinDirectory(directory string, inputs ...streamInputPath) error {
	guardInputs := make([]pathguard.Input, len(inputs))
	for i, input := range inputs {
		guardInputs[i] = pathguard.Input{Flag: input.flag, Path: input.path}
	}
	return pathguard.RejectInputsWithinDirectory(directory, guardInputs...)
}

func (localStreamService) ValidateFFmpeg(_ context.Context, ffmpeg string) error {
	if strings.TrimSpace(ffmpeg) == "" {
		return fmt.Errorf("ffmpeg is not configured")
	}
	if _, err := exec.LookPath(ffmpeg); err != nil {
		return fmt.Errorf("ffmpeg %q is not accessible: %w", ffmpeg, err)
	}
	return nil
}
