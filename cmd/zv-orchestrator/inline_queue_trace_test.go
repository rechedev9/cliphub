package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rechedev9/cliphub/internal/obs"
)

func TestInlineTraceCorrelatesDistinctRetriesAndPreservesOutcome(t *testing.T) {
	var output bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(prior)
	job := uuid.NewString()
	task := asynq.NewTask("render:variant", []byte(`{"job_id":"`+job+`"}`))
	queue := newInlineQueue(nil, 1)
	queued := inlineTask{task: task, policy: inlineTaskPolicy{maxRetries: 1}}
	calls := 0
	err, started := queue.handle(context.Background(), queued, func(ctx context.Context, _ *asynq.Task) error {
		calls++
		trace := obs.TraceFrom(ctx)
		if trace.JobID != job || trace.Attempt != calls || trace.AttemptID == "" {
			t.Errorf("invalid context: %+v", trace)
		}
		if calls == 1 {
			return errors.New("first encoder failure")
		}
		return nil
	})
	if err != nil || !started || calls != 2 {
		t.Fatalf("retry behavior changed: %v %v %d", err, started, calls)
	}
	var entries []obs.TraceEntry
	for _, line := range strings.Split(output.String(), "\n") {
		_, body, ok := strings.Cut(line, obs.TracePrefix)
		if !ok {
			continue
		}
		var entry obs.TraceEntry
		if err := json.Unmarshal([]byte(body), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 4 || entries[0].Event != "attempt.started" || entries[1].Outcome != "error" || entries[2].Event != "attempt.started" || entries[3].Outcome != "ok" {
		t.Fatalf("lifecycle: %s", output.String())
	}
	if entries[0].AttemptID != entries[1].AttemptID || entries[2].AttemptID != entries[3].AttemptID || entries[0].AttemptID == entries[2].AttemptID {
		t.Fatalf("attempt identity lost: %+v", entries)
	}
}

func TestInlineTraceRecordsPanicAndStillPanics(t *testing.T) {
	var output bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(prior)
	defer func() {
		if recover() != "canary panic" {
			t.Error("panic was swallowed or changed")
		}
		for _, value := range []string{"attempt.started", "attempt.finished", "canary panic", "goroutine", `"outcome":"error"`} {
			if !strings.Contains(output.String(), value) {
				t.Errorf("missing %s: %s", value, output.String())
			}
		}
	}()
	_ = executeTracedTask(context.Background(), asynq.NewTask("canary", nil), func(context.Context, *asynq.Task) error { panic("canary panic") })
}
