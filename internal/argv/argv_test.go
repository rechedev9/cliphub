package argv

import (
	"bytes"
	"context"
	"testing"
)

func TestCommandRunsArgv(t *testing.T) {
	cmd := Command("go", "version")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("expected go version output")
	}
}

func TestCommandContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := CommandContext(ctx, "go", "version")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected canceled command to fail")
	}
}
