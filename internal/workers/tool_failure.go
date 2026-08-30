package workers

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

const (
	toolCombinedOutputLimit      = 4096
	emptyCombinedOutputMarker    = "CombinedOutput is empty"
	truncatedCombinedOutputMark  = "...(truncated)"
	combinedOutputPresenceMarker = "CombinedOutput"
)

func toolBaseName(exe string) string {
	return filepath.Base(strings.ReplaceAll(strings.TrimSpace(exe), "\\", "/"))
}

func formatCombinedOutput(out []byte) string {
	text := strings.Join(strings.Fields(string(out)), " ")
	if text == "" {
		return emptyCombinedOutputMarker
	}
	if len(text) > toolCombinedOutputLimit {
		return text[:toolCombinedOutputLimit] + truncatedCombinedOutputMark
	}
	return text
}

func formatToolFailure(exe string, out []byte, err error) error {
	if err == nil {
		return nil
	}
	text := formatCombinedOutput(out)
	log.Printf("tool=%s combined_output=%s", toolBaseName(exe), text)
	return fmt.Errorf("%s failed: %w: %s", exe, err, text)
}

func toolFailureIncludesOutput(err error) bool {
	return err != nil && strings.Contains(err.Error(), combinedOutputPresenceMarker)
}

func ensureToolFailureOutput(exe string, out []byte, err error) error {
	if err == nil {
		return nil
	}
	if toolFailureIncludesOutput(err) {
		return err
	}
	return formatToolFailure(exe, out, err)
}
