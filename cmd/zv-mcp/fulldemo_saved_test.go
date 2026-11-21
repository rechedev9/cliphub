package main

import (
	"strings"
	"testing"
)

func TestDescribeCaptureCheckSaysWhatCS2RecordsAndWarnsOnLowSpace(t *testing.T) {
	kept := describeCaptureCheck("job", fullDemoCapture{})
	if kept.NeedsCapture || !strings.Contains(kept.Summary, "solo se vuelve a montar") || kept.PlanHash != "" {
		t.Errorf("kept captures = %+v", kept)
	}
	capture := fullDemoCapture{Needed: true, MissingRounds: []int{3, 7}}
	capture.Space = &struct {
		FreeBytes   int64 `json:"free_bytes"`
		NeededBytes int64 `json:"needed_bytes"`
		Short       bool  `json:"short"`
	}{FreeBytes: 1 << 30, NeededBytes: 5 << 30, Short: true}
	out := describeCaptureCheck("job", capture)
	if !out.NeedsCapture || !strings.Contains(out.Summary, "2 rondas") || out.LowSpace == "" {
		t.Errorf("missing rounds = %+v", out)
	}
}
