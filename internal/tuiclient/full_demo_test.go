package tuiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFullDemoClientBoundedJSONAndSafePaths(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"one document", `{"document":null}`, true},
		{"trailing document", `{} {}`, false},
		{"truncated", `{`, false},
		{"oversized", `{"value":"` + strings.Repeat("a", 8<<20) + `"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.body) })
			_, err := client.GetFullDemoPlan(t.Context(), "e16560b4-6ee2-48a6-8e40-15e5d712856d")
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid path reached HTTP") })
	for _, id := range []string{"../other", "not-uuid", "00000000-0000-0000-0000-000000000000"} {
		t.Run(id, func(t *testing.T) {
			if _, err := client.PlanFullDemo(t.Context(), id, json.RawMessage(`{}`)); err == nil {
				t.Fatal("invalid ID accepted")
			}
		})
	}
	if _, err := client.GetFullDemoEvidence(t.Context(), "e16560b4-6ee2-48a6-8e40-15e5d712856d", "../../source.dem"); err == nil {
		t.Fatal("invalid evidence path accepted")
	}
}

func TestFullDemoUploadWriterStopsOnRejectionOrCancellation(t *testing.T) {
	for _, stop := range []string{"rejected", "cancelled"} {
		t.Run(stop, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "upload.wav")
			if err := os.WriteFile(file, []byte(strings.Repeat("fixture", 1<<20)), 0600); err != nil {
				t.Fatal(err)
			}
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"code":"unauthorized","error":"missing token"}`)
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if stop == "cancelled" {
				cancel()
			}
			_, err := client.UploadFullDemoAsset(ctx, file, json.RawMessage(`{"title":"fixture"}`))
			if err == nil {
				t.Fatal("upload unexpectedly accepted")
			}
			if stop == "rejected" && StatusCode(err) != http.StatusUnauthorized {
				t.Fatalf("rejection: %v", err)
			}
			if stop == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
}
