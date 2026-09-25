package job

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestStatusWireNames pins the canonical name of every status in both
// directions: String, ParseStatus and the JSON encoding all use it.
func TestStatusWireNames(t *testing.T) {
	cases := map[Status]string{
		StatusQueued:         "queued",
		StatusParsing:        "parsing",
		StatusParsed:         "parsed",
		StatusRecording:      "recording",
		StatusRecorded:       "recorded",
		StatusComposing:      "composing",
		StatusComposed:       "composed",
		StatusDone:           "done",
		StatusFailed:         "failed",
		StatusScanning:       "scanning",
		StatusScanned:        "scanned",
		StatusReviewRequired: "review_required",
	}
	if len(cases) != len(Statuses()) {
		t.Fatalf("wire names cover %d statuses, Statuses() has %d", len(cases), len(Statuses()))
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", s, got, want)
		}
		if parsed, err := ParseStatus(want); err != nil || parsed != s {
			t.Errorf("ParseStatus(%q) = (%v, %v), want %v", want, parsed, err, s)
		}
		b, err := json.Marshal(s)
		if err != nil || string(b) != `"`+want+`"` {
			t.Errorf("json.Marshal(%v) = (%s, %v), want %q", s, b, err, want)
		}
		var decoded Status
		if err := json.Unmarshal(b, &decoded); err != nil || decoded != s {
			t.Errorf("json.Unmarshal(%s) = (%v, %v), want %v", b, decoded, err, s)
		}
	}
}

func TestStatusRejectsUnknownName(t *testing.T) {
	if _, err := ParseStatus("bogus"); err == nil {
		t.Error("ParseStatus(bogus) error = nil, want error")
	}
	var s Status
	if err := json.Unmarshal([]byte(`"bogus"`), &s); err == nil {
		t.Error("Unmarshal(\"bogus\") error = nil, want error")
	}
}

func TestCanHaveRenderStateCoversEveryStatus(t *testing.T) {
	want := map[Status]bool{
		StatusQueued:         false,
		StatusParsing:        false,
		StatusParsed:         false,
		StatusRecording:      false,
		StatusRecorded:       true,
		StatusComposing:      true,
		StatusComposed:       true,
		StatusDone:           true,
		StatusFailed:         true,
		StatusScanning:       false,
		StatusScanned:        false,
		StatusReviewRequired: true,
	}
	for _, s := range Statuses() {
		expected, ok := want[s]
		if !ok {
			t.Fatalf("Statuses() includes %s with no CanHaveRenderState expectation", s)
		}
		if got := s.CanHaveRenderState(); got != expected {
			t.Errorf("%s.CanHaveRenderState() = %v, want %v", s, got, expected)
		}
	}
}

func TestJobMarshalsToExpectedShape(t *testing.T) {
	j := Job{
		ID:            uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Status:        StatusQueued,
		DemoPath:      "/tmp/x.dem",
		DemoSHA256:    "abc",
		TargetSteamID: "76561198000000000",
		CreatedAt:     time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	out := string(b)
	if !strings.Contains(out, `"status":"queued"`) {
		t.Errorf("status not rendered as string: %s", out)
	}
	if !strings.Contains(out, `"id":"11111111-1111-1111-1111-111111111111"`) {
		t.Errorf("id not rendered as UUID string: %s", out)
	}
	if strings.Contains(out, "failure_code") || strings.Contains(out, "failure_reason") {
		t.Errorf("empty failure fields should be omitted: %s", out)
	}
}
