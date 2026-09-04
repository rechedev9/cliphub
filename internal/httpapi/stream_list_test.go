package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestStreamJobListItemsCarryClipCountWithoutThePlan(t *testing.T) {
	plan := json.RawMessage(`{"schema_version":"1","variant":"streamer-fullframe-nocam","clips":[{"id":"c1","start_seconds":0,"end_seconds":5},{"id":"c2","start_seconds":7,"end_seconds":9}]}`)
	cases := []struct {
		name      string
		plan      json.RawMessage
		wantCount int
	}{
		{name: "two cuts", plan: plan, wantCount: 2},
		{name: "no plan", plan: nil, wantCount: 0},
		{name: "empty clips", plan: json.RawMessage(`{"clips":[]}`), wantCount: 0},
		{name: "malformed plan counts as empty", plan: json.RawMessage(`{"clips":`), wantCount: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := streamJobListItems([]streamclips.Job{{ID: uuid.New(), Status: streamclips.StatusReady, EditPlan: tc.plan}})
			if len(items) != 1 || items[0].ClipCount != tc.wantCount {
				t.Fatalf("items = %+v, want one row with clip_count %d", items, tc.wantCount)
			}
			b, err := json.Marshal(items)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "edit_plan") || strings.Contains(string(b), "start_seconds") {
				t.Fatalf("list row must not carry the plan: %s", b)
			}
			if !strings.Contains(string(b), `"clip_count":`) {
				t.Fatalf("list row must carry clip_count: %s", b)
			}
		})
	}
}
