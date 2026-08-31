package demooverlay

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed intro-chrome.png
var introChromePNG []byte

//go:embed outro-chrome.png
var outroChromePNG []byte

func writeChrome(path string, data []byte) error {
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		return fmt.Errorf("full-demo chrome is not a PNG")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write full-demo chrome: %w", err)
	}
	return nil
}
