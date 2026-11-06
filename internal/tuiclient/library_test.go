package tuiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The server applies only the tags read before a demo part, so tags must come first.
func TestImportLibraryDemoSendsTagsBeforeTheFile(t *testing.T) {
	demo := filepath.Join(t.TempDir(), "match.dem")
	if err := os.WriteFile(demo, []byte("PBDEMS2fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart: %v", err)
		}
		var order []string
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("next part: %v", err)
			}
			b, _ := io.ReadAll(part)
			if part.FormName() == "tag" {
				order = append(order, "tag:"+string(b))
			} else {
				order = append(order, part.FormName())
			}
		}
		if want := []string{"tag:ESL", "tag:scrim", "demo"}; !slices.Equal(order, want) {
			t.Errorf("parts = %v, want %v", order, want)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"demos": []map[string]any{{"status": "imported", "demo": map[string]any{"id": "d1", "tags": []string{"ESL", "scrim"}}}}})
	})
	res, err := c.ImportLibraryDemo(context.Background(), demo, []string{"ESL", "scrim"})
	if err != nil {
		t.Fatalf("ImportLibraryDemo: %v", err)
	}
	if res.Demo == nil || !slices.Equal(res.Demo.Tags, []string{"ESL", "scrim"}) {
		t.Fatalf("result = %+v", res)
	}
}
