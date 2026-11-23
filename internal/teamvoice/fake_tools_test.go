package teamvoice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The test binary stands in for nemo-speech when this variable is set.
const fakeNemoEnv = "CLIPHUB_FAKE_NEMO"

const (
	fakeNemoRunsEnv   = "CLIPHUB_FAKE_NEMO_RUNS"
	fakeNemoVulkanEnv = "CLIPHUB_FAKE_NEMO_VULKAN"
)

type fakeWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func TestMain(m *testing.M) {
	if words, ok := os.LookupEnv(fakeNemoEnv); ok && len(os.Args) > 1 && os.Args[1] == "transcribe" {
		os.Exit(fakeNemo(words))
	}
	os.Exit(m.Run())
}

func argAfter(flag string) string {
	if i := slices.Index(os.Args, flag); i >= 0 && i+1 < len(os.Args) {
		return os.Args[i+1]
	}
	return ""
}

// fakeNemo answers each WAV of the input folder, in name order, with the next word list of words.
func fakeNemo(words string) int {
	if code := fakeNemoDevice(); code != 0 {
		return code
	}
	if words == "fail" {
		fmt.Fprintln(os.Stderr, "model load failed")
		return 3
	}
	var lists [][]fakeWord
	if err := json.Unmarshal([]byte(words), &lists); err != nil {
		return 4
	}
	inputs, _ := filepath.Glob(filepath.Join(os.Args[2], "*.wav"))
	slices.Sort(inputs)
	for i, in := range inputs {
		out := struct {
			File  string     `json:"file"`
			Text  string     `json:"text"`
			Words []fakeWord `json:"words"`
		}{File: in, Words: []fakeWord{}}
		if i < len(lists) {
			out.Words = lists[i]
		}
		for _, w := range out.Words {
			out.Text = strings.TrimSpace(out.Text + " " + w.Word)
		}
		raw, _ := json.Marshal(out)
		name := strings.TrimSuffix(filepath.Base(in), ".wav") + ".json"
		if err := os.WriteFile(filepath.Join(argAfter("--output-dir"), name), raw, 0o600); err != nil {
			return 5
		}
	}
	return 0
}

func fakeNemoDevice() int {
	device := argAfter("--device")
	if runs := os.Getenv(fakeNemoRunsEnv); runs != "" {
		exe, err := os.Executable()
		if err != nil {
			return 6
		}
		f, err := os.OpenFile(runs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 6
		}
		_, writeErr := fmt.Fprintf(f, "%s %s\n", exe, device)
		if errors.Join(writeErr, f.Close()) != nil {
			return 6
		}
	}
	if device == "vulkan" && os.Getenv(fakeNemoVulkanEnv) == "fail" {
		fmt.Fprintln(os.Stderr, "ggml_vulkan: vk::DeviceLostError")
		return 7
	}
	return 0
}

// fakeDecoder hands over fixed PCM for any recording.
type fakeDecoder []byte

func (d fakeDecoder) RawPCM(_ context.Context, job PCMJob) error {
	return os.WriteFile(job.Dst, d, 0o600)
}

func fakeTool(t *testing.T) (cli, model string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	model = filepath.Join(t.TempDir(), "model.bin")
	if err := os.WriteFile(model, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return self, model
}
