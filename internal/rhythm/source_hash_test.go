package rhythm

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceSHA256StreamsCompleteSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.mp4")
	content := make([]byte, 3*1024*1024+17)
	for i := range content {
		content[i] = byte(i % 251)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := SourceSHA256(path)
	if err != nil {
		t.Fatalf("SourceSHA256 error = %v", err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256(content))
	if got != want {
		t.Fatalf("SourceSHA256 = %q, want %q", got, want)
	}
}

func TestSourceSHA256PreservesMissingFileCause(t *testing.T) {
	_, err := SourceSHA256(filepath.Join(t.TempDir(), "missing.mp4"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SourceSHA256 error = %v, want os.ErrNotExist", err)
	}
}

func TestDecodeStableMonoSamplesBindsHashToDecodedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.mp4")
	content := []byte("stable source bytes")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	wantSamples := []float64{0.25, -0.5}
	samples, gotHash, err := decodeStableMonoSamples(
		context.Background(),
		"ffmpeg",
		path,
		22050,
		func(_ context.Context, _ string, inputPath string, _ int) ([]float64, error) {
			decoded, readErr := os.ReadFile(inputPath)
			if readErr != nil {
				return nil, readErr
			}
			if string(decoded) != string(content) {
				t.Fatalf("decoder read %q, want %q", decoded, content)
			}
			return wantSamples, nil
		},
	)
	if err != nil {
		t.Fatalf("decodeStableMonoSamples error = %v", err)
	}
	if len(samples) != len(wantSamples) || samples[0] != wantSamples[0] || samples[1] != wantSamples[1] {
		t.Fatalf("samples = %v, want %v", samples, wantSamples)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256(content))
	if gotHash != wantHash {
		t.Fatalf("source hash = %q, want %q", gotHash, wantHash)
	}
}

func TestDecodeStableMonoSamplesRejectsSourceMutationDuringDecode(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "modify in place",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("modified source bytes"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "replace path",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				replacement := path + ".replacement"
				if err := os.WriteFile(replacement, []byte("replacement source bytes"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, path); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.mp4")
			original := []byte("original source bytes")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}

			_, _, err := decodeStableMonoSamples(
				context.Background(),
				"ffmpeg",
				path,
				22050,
				func(_ context.Context, _ string, inputPath string, _ int) ([]float64, error) {
					decoded, readErr := os.ReadFile(inputPath)
					if readErr != nil {
						return nil, readErr
					}
					if string(decoded) != string(original) {
						t.Fatalf("decoder read %q, want original %q", decoded, original)
					}
					tt.mutate(t, inputPath)
					return []float64{0}, nil
				},
			)
			if err == nil || !strings.Contains(err.Error(), "input changed during audio analysis") {
				t.Fatalf("decodeStableMonoSamples error = %v, want source mutation error", err)
			}
		})
	}
}
