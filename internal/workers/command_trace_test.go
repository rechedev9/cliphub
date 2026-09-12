package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/obs"
)

func TestDiagnosticCommandHelper(t *testing.T) {
	if os.Getenv("CLIPHUB_DIAGNOSTIC_COMMAND_HELPER") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString("stdout result must remain available to the caller\n")
	_, _ = os.Stderr.WriteString("Preparing the encoder\nCANARY_CAUSE: encoder device unavailable\n")
	os.Exit(17)
}

func TestCommandTraceRetainsRealSubprocessCauseAndExitCode(t *testing.T) {
	t.Setenv("CLIPHUB_DIAGNOSTIC_COMMAND_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(prior)
	trace := obs.TraceContext{JobID: uuid.NewString(), AttemptID: uuid.NewString(), Operation: "render:variant", Attempt: 2}
	ctx := obs.WithTrace(context.Background(), trace)
	combined, err := (execCommandRunner{}).Run(ctx, executable, "-test.run=^TestDiagnosticCommandHelper$")
	if err == nil || !strings.Contains(err.Error(), "CANARY_CAUSE: encoder device unavailable") || !strings.Contains(string(combined), "stdout result must remain") {
		t.Fatalf("changed command failure/output: %s %v", combined, err)
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
		if entry.JobID != trace.JobID || entry.AttemptID != trace.AttemptID || entry.Operation != trace.Operation || entry.Attempt != 2 {
			t.Fatalf("lost correlation: %+v", entry)
		}
	}
	if len(entries) != 4 || entries[0].Event != "process.started" || entries[1].Event != "process.stderr" || !strings.Contains(entries[2].Message, "CANARY_CAUSE") || entries[3].Event != "process.finished" || entries[3].ExitCode == nil || *entries[3].ExitCode != 17 {
		t.Fatalf("missing diagnostic lifecycle: %s", output.String())
	}
	if strings.Contains(output.String(), "stdout result must remain") || strings.Contains(output.String(), "-test.run") {
		t.Fatalf("logged stdout transport or argv: %s", output.String())
	}
}

// Invoked only by the cross-language remote-debug integration harness. The
// command runner, trace writer, pipe reader, spool and collector are real.
func TestDiagnosticEndToEndCanary(t *testing.T) {
	if os.Getenv("CLIPHUB_DIAGNOSTIC_END_TO_END") != "1" {
		return
	}
	id, err := uuid.Parse(os.Getenv("CLIPHUB_DIAGNOSTIC_JOB_ID"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLIPHUB_DIAGNOSTIC_COMMAND_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := obs.WithTrace(context.Background(), obs.TraceContext{JobID: id.String(), AttemptID: uuid.NewString(), Operation: "render:variant", Attempt: 1})
	obs.EmitTrace(ctx, obs.TraceEntry{Event: "attempt.started", Message: "Remote debugging validation canary started"})
	_, err = (execCommandRunner{}).Run(ctx, executable, "-test.run=^TestDiagnosticCommandHelper$")
	if err == nil {
		t.Fatal("the canary subprocess unexpectedly succeeded")
	}
	// Deliberately terse, matching the original incident. The remote stderr
	// must still be enough to diagnose it without the local studio.log.
	obs.EmitTrace(ctx, obs.TraceEntry{Event: "attempt.finished", Level: "error", Message: "exit status 1", Outcome: "error"})
}
