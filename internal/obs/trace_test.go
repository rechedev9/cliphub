package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestTraceWriterRestoresChildToolEvidenceWithParentCorrelation(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	parent := TraceContext{JobID: uuid.NewString(), AttemptID: uuid.NewString(), Operation: "render:variant", Attempt: 2}
	code := int64(17)
	child := TraceEntry{TraceContext: TraceContext{JobID: uuid.NewString()}, Event: "tool.finished", Level: "error", Message: "ffmpeg loudness: encoder no disponible 🙂", Outcome: "error", ExitCode: &code}
	encoded, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	writer := NewTraceWriter(WithTrace(context.Background(), parent), "editor")
	line := []byte("2026/09/12 09:00:00 " + TracePrefix + string(encoded) + "\n")
	for _, value := range line {
		if _, err := writer.Write([]byte{value}); err != nil {
			t.Fatal(err)
		}
	}
	_ = writer.Close()
	_, body, ok := strings.Cut(strings.TrimSpace(output.String()), TracePrefix)
	if !ok {
		t.Fatal(output.String())
	}
	var result TraceEntry
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	if result.TraceContext != parent || result.Level != "error" || result.Event != "tool.finished" || result.Message != child.Message || result.ExitCode == nil || *result.ExitCode != 17 {
		t.Fatalf("child evidence/correlation lost: %+v", result)
	}
}
