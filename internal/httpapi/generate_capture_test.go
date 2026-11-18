package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// captureCheckFunc stands in for the record worker's reuse check.
type captureCheckFunc func(context.Context, *asynq.Task) (bool, error)

func (f captureCheckFunc) CaptureNeeded(ctx context.Context, t *asynq.Task) (bool, error) {
	return f(ctx, t)
}

// A page load sends allow_capture false: a generate whose record task would open CS2 is
// refused before anything is queued. A click and zv-mcp send no flag and record as before.
func TestGenerateThatForbidsACaptureNeverRecords(t *testing.T) {
	cases := []struct {
		name    string
		flag    string
		needed  bool
		err     error
		noCheck bool
		want    int
		checked bool
	}{
		{name: "clips freed or not reusable", flag: `,"allow_capture":false`, needed: true, want: http.StatusConflict, checked: true},
		{name: "clips reusable", flag: `,"allow_capture":false`, want: http.StatusAccepted, checked: true},
		{name: "check fails", flag: `,"allow_capture":false`, err: errors.New("disk read failed"), want: http.StatusConflict, checked: true},
		{name: "no record worker to ask", flag: `,"allow_capture":false`, noCheck: true, want: http.StatusConflict},
		{name: "a click", needed: true, want: http.StatusAccepted},
		{name: "a click that says so", flag: `,"allow_capture":true`, needed: true, want: http.StatusAccepted},
	}
	for _, tc := range cases {
		h, j := horizontalFixture(t)
		queue := &fakeQueue{}
		h.queue = queue
		var checked *asynq.Task
		if !tc.noCheck {
			WithCaptureCheck(captureCheckFunc(func(_ context.Context, task *asynq.Task) (bool, error) {
				checked = task
				return tc.needed, tc.err
			}))(h)
		}

		rw := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","segment_ids":["seg-001"],"edit":{"format":"short-9x16"}`+tc.flag+`}`)

		if rw.Code != tc.want {
			t.Fatalf("%s: status = %d %s, want %d", tc.name, rw.Code, rw.Body.String(), tc.want)
		}
		if (checked != nil) != tc.checked {
			t.Fatalf("%s: capture check consulted = %v, want %v", tc.name, checked != nil, tc.checked)
		}
		if tc.want == http.StatusAccepted {
			if len(queue.enqueued) != 1 || (tc.checked && checked != queue.enqueued[0]) {
				t.Fatalf("%s: enqueued %d tasks, want the checked record task", tc.name, len(queue.enqueued))
			}
			if intent, _, err := tasks.GenerateIntentFromTask(queue.enqueued[0]); err != nil || intent.NoCapture != tc.checked {
				t.Fatalf("%s: queued task may capture = %v (%v), want %v", tc.name, !intent.NoCapture, err, !tc.checked)
			}
			continue
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil || body.Code != "capture_required" {
			t.Fatalf("%s: body = %s, want code capture_required", tc.name, rw.Body.String())
		}
		if _, found, err := h.readGenerateIntent(j.ID); len(queue.enqueued) != 0 || found || err != nil {
			t.Fatalf("%s: refused generate queued %d tasks or wrote its intent (%v, %v)", tc.name, len(queue.enqueued), found, err)
		}
		if got := h.repo.(*fakeRepo).jobs[j.ID].Status; got != job.StatusDone {
			t.Fatalf("%s: job status = %s, want done", tc.name, got)
		}
	}
}

// A click must record, so it never joins a queued task that may not capture: it waits for it instead.
func TestAClickNeverJoinsAQueuedTaskThatMayNotCapture(t *testing.T) {
	h, j := horizontalFixture(t)
	h.queue = &uniqueScopeQueue{}
	WithCaptureCheck(captureCheckFunc(func(context.Context, *asynq.Task) (bool, error) { return false, nil }))(h)
	const body = `{"preset":"clean-pov-60","segment_ids":["seg-001"],"edit":{"format":"short-9x16"}`
	if rw := postGenerate(t, h, j.ID, body+`,"allow_capture":false}`); rw.Code != http.StatusAccepted {
		t.Fatalf("page load = %d %s, want 202", rw.Code, rw.Body.String())
	}
	if rw := postGenerate(t, h, j.ID, body+`}`); rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), generateWorkActive) {
		t.Fatalf("click = %d %s, want 409 %s", rw.Code, rw.Body.String(), generateWorkActive)
	}
}
