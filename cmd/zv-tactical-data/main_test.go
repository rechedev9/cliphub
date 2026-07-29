package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOutputPathRejectsDemoAlias(t *testing.T) {
	demo := filepath.Join(t.TempDir(), "match.dem")
	if err := os.WriteFile(demo, []byte("PBDEMS2"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := validateOutputPath(demo, demo)
	if err == nil || !strings.Contains(err.Error(), "--demo") {
		t.Fatalf("validateOutputPath error = %v, want demo alias rejection", err)
	}
}
