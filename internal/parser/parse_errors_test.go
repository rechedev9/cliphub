package parser

import (
	"bytes"
	"strings"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
)

// TestParseToEndRecoversCorruptDemoPanic reproduces a real incident: a
// header-valid but otherwise garbage payload (e.g. a cloud portal submission
// that isn't truly a CS2 demo despite passing the magic-byte check) made
// demoinfocs.Parser.ParseToEnd panic with invalid protobuf wire-format data,
// which — unrecovered — crashed the whole orchestrator process instead of
// just failing this one job.
func TestParseToEndRecoversCorruptDemoPanic(t *testing.T) {
	garbage := bytes.Repeat([]byte{0x01}, 4096)
	demo := append([]byte("PBDEMS2\x00"), garbage...)

	p := demoinfocs.NewParser(bytes.NewReader(demo))
	defer p.Close()

	err := parseToEnd(p)
	if err == nil {
		t.Fatal("parseToEnd error = nil, want a recovered error for a corrupt demo")
	}
	if !strings.HasPrefix(err.Error(), "demo_incompatible:") {
		t.Fatalf("parseToEnd error = %q, want it to start with %q", err.Error(), "demo_incompatible:")
	}
}
