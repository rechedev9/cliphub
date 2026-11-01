package tuiclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestSongAudioReturnsBytesAndMime(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/songs/phonk-01/audio" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/mpeg; charset=binary")
		_, _ = w.Write([]byte("abc"))
	})
	data, mime, err := c.SongAudio(context.Background(), "phonk-01", 10)
	if err != nil || string(data) != "abc" || mime != "audio/mpeg" {
		t.Fatalf("got %q %q %v", data, mime, err)
	}
}

func TestSongAudioStopsAboveCap(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	})
	if _, _, err := c.SongAudio(context.Background(), "big", 10); !errors.Is(err, ErrAudioTooLarge) {
		t.Fatalf("got %v, want ErrAudioTooLarge", err)
	}
}
