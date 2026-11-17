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
	joined := json.RawMessage(`{"clips":[{"id":"c1","start_seconds":0,"end_seconds":5},{"id":"c2","start_seconds":7,"end_seconds":9},{"id":"c3","start_seconds":12,"end_seconds":15}],"groups":[{"id":"g1","clip_ids":["c1","c3"]}]}`)
	cases := []struct {
		name       string
		plan       json.RawMessage
		wantCount  int
		wantShorts int
	}{
		{name: "two cuts", plan: plan, wantCount: 2, wantShorts: 2},
		{name: "a joined group is one Short", plan: joined, wantCount: 3, wantShorts: 2},
		{name: "no plan", plan: nil, wantCount: 0},
		{name: "empty clips", plan: json.RawMessage(`{"clips":[]}`), wantCount: 0},
		{name: "malformed plan counts as empty", plan: json.RawMessage(`{"clips":`), wantCount: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
			items := h.streamJobListItems([]streamclips.Job{{ID: uuid.New(), Status: streamclips.StatusReady, EditPlan: tc.plan}})
			if len(items) != 1 || items[0].ClipCount != tc.wantCount || items[0].ShortCount != tc.wantShorts {
				t.Fatalf("items = %+v, want one row with clip_count %d and short_count %d", items, tc.wantCount, tc.wantShorts)
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
