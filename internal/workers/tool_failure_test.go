package workers

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFormatCombinedOutput(t *testing.T) {
	cases := []struct {
		name string
		out  []byte
		want string
	}{
		{name: "nil", want: emptyCombinedOutputMarker},
		{name: "blank", out: []byte("  \n\t"), want: emptyCombinedOutputMarker},
		{name: "keeps stderr text", out: []byte(" missing manifest\n"), want: "missing manifest"},
		{name: "collapses multiline dumps", out: []byte("line one\n  line two\n"), want: "line one line two"},
		{
			name: "truncates long dumps",
			out:  bytes.Repeat([]byte("a"), toolCombinedOutputLimit+32),
			want: strings.Repeat("a", toolCombinedOutputLimit) + truncatedCombinedOutputMark,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatCombinedOutput(tc.out); got != tc.want {
				t.Fatalf("formatCombinedOutput = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatToolFailureAlwaysIncludesCombinedOutput(t *testing.T) {
	cases := []struct {
		name       string
		out        []byte
		wantInErr  string
		wantInLog  string
		wantAbsent string
	}{
		{
			name:      "empty output is an explicit fact",
			wantInErr: emptyCombinedOutputMarker,
			wantInLog: "tool=zv-editor.exe combined_output=" + emptyCombinedOutputMarker,
		},
		{
			name:      "keeps captured stderr",
			out:       []byte("open edit-document.json: file does not exist\n"),
			wantInErr: "open edit-document.json: file does not exist",
			wantInLog: "tool=zv-editor.exe combined_output=open edit-document.json: file does not exist",
		},
		{
			name:       "truncates ffmpeg-sized dumps",
			out:        bytes.Repeat([]byte("f"), toolCombinedOutputLimit+64),
			wantInErr:  truncatedCombinedOutputMark,
			wantInLog:  truncatedCombinedOutputMark,
			wantAbsent: strings.Repeat("f", toolCombinedOutputLimit+1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			log.SetFlags(0)
			t.Cleanup(func() { log.SetOutput(os.Stderr) })

			err := formatToolFailure("C:\\Studio\\tools\\zv-editor.exe", tc.out, errors.New("exit status 1"))
			if err == nil {
				t.Fatal("formatToolFailure = nil, want wrapped exit")
			}
			got := err.Error()
			if !strings.Contains(got, "zv-editor.exe failed") {
				t.Fatalf("error = %q, want exe prefix", got)
			}
			if !strings.Contains(got, "exit status 1") {
				t.Fatalf("error = %q, want exit status", got)
			}
			if !strings.Contains(got, tc.wantInErr) {
				t.Fatalf("error = %q, want %q", got, tc.wantInErr)
			}
			if tc.wantAbsent != "" && strings.Contains(got, tc.wantAbsent) {
				t.Fatalf("error still contains untruncated dump")
			}
			logged := buf.String()
			if !strings.Contains(logged, tc.wantInLog) {
				t.Fatalf("log = %q, want %q", logged, tc.wantInLog)
			}
		})
	}
}

func TestEnsureToolFailureOutputDoesNotDoubleWrap(t *testing.T) {
	first := formatToolFailure("zv-editor.exe", nil, errors.New("exit status 1"))
	second := ensureToolFailureOutput("zv-editor.exe", nil, first)
	if second != first {
		t.Fatalf("ensureToolFailureOutput rewrapped %q into %q", first, second)
	}
}

func TestExecCommandRunnerPersistsEmptyCombinedOutput(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false is not available")
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	out, runErr := execCommandRunner{}.Run(context.Background(), falsePath)
	if runErr == nil {
		t.Fatal("Run error = nil, want exit status 1")
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("CombinedOutput = %q, want empty", out)
	}
	if !strings.Contains(runErr.Error(), emptyCombinedOutputMarker) {
		t.Fatalf("error = %q, want %q", runErr, emptyCombinedOutputMarker)
	}
	if !strings.Contains(buf.String(), "combined_output="+emptyCombinedOutputMarker) {
		t.Fatalf("log = %q, want combined_output marker", buf.String())
	}
}
