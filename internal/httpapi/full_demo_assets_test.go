package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type fullDemoAssetFailureStorage struct {
	*fakeStorage
	suffix string
}

func (s fullDemoAssetFailureStorage) Put(key string, body io.Reader) error {
	if s.suffix != "" && strings.HasSuffix(key, s.suffix) {
		return errors.New("injected immutable upload failure")
	}
	return s.fakeStorage.Put(key, body)
}

type fullDemoAssetProber struct{ duration float64 }

func (p fullDemoAssetProber) Probe(context.Context, string) (streamclips.SourceProbe, error) {
	return streamclips.SourceProbe{DurationSeconds: p.duration, AudioCodec: "pcm_s16le"}, nil
}

func TestFullDemoAssetImportPublishesOnlyVerifiedCompleteRows(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("FFmpeg required for asset upload contracts")
	}
	fixture := filepath.Join(t.TempDir(), "tone.wav")
	_, err = exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=f=880:r=48000:d=0.25", "-c:a", "pcm_s16le", fixture).Output()
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"valid audio", "media upload failure", "provenance upload failure", "corrupt file", "cancelled", "nonfinite duration", "content dedup", "replaced bytes"} {
		t.Run(name, func(t *testing.T) {
			backing := newFakeStorage()
			storage := fullDemoAssetFailureStorage{fakeStorage: backing}
			switch name {
			case "media upload failure":
				storage.suffix = "media.mp4"
			case "provenance upload failure":
				storage.suffix = "provenance.json"
			}
			assets := newFakeEditorAssets()
			h := NewHandlers(newFakeRepo(), storage, &fakeQueue{}, WithEditorRepositories(assets, newFakeEditorProjects()))
			duration := .25
			if name == "nonfinite duration" {
				duration = math.NaN()
			}
			h.streamProber = fullDemoAssetProber{duration: duration}
			r := httptest.NewRequest("POST", "/api/editor/assets", nil)
			if name == "cancelled" {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			body := pcm
			if name == "corrupt file" {
				body = []byte("invalid media")
			}
			provenance := &mediaassets.Provenance{SchemaVersion: "1.0", Title: "Synthetic tone", Creator: "ClipHub tests", SourceURL: "local:synthetic-tone", Permission: "Original synthetic fixture"}
			got, err := h.ingestEditorAsset(r, bytes.NewReader(body), "tone.wav", mediaassets.OriginUpload, nil, "", "", provenance)
			valid := name == "valid audio" || name == "content dedup" || name == "replaced bytes"
			if (err == nil) != valid {
				t.Fatalf("import: %v", err)
			}
			if !valid {
				if len(assets.assets) != 0 {
					t.Fatal("failed import became selectable")
				}
				return
			}
			declared, found, err := mediaassets.LoadProvenance(backing, got.ID)
			if err != nil || !found || declared.AssetSHA256 != got.SHA256 {
				t.Fatalf("provenance: %+v %v", declared, err)
			}
			if name == "content dedup" || name == "replaced bytes" {
				if name == "replaced bytes" {
					backing.puts[got.MediaKey] = []byte("replaced")
				}
				next, err := h.ingestEditorAsset(r, bytes.NewReader(pcm), "tone.wav", mediaassets.OriginUpload, nil, "", "", provenance)
				if err != nil {
					t.Fatal(err)
				}
				if (next.ID == got.ID) != (name == "content dedup") {
					t.Fatal("asset reuse ignored actual content identity")
				}
			}
		})
	}
}

func TestFullDemoRenderEnvelopeBounds(t *testing.T) {
	for _, tc := range []struct {
		name          string
		bytes         int
		render, valid bool
	}{
		{"legacy small", 100, false, true}, {"legacy cap", 2 << 20, false, false},
		{"render snapshot envelope", 5 << 20, true, true}, {"render cap", 9 << 20, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{"value":"`+strings.Repeat("x", tc.bytes)+`"}`))
			var body struct {
				Value string `json:"value"`
			}
			var err error
			if tc.render {
				err = decodeRenderJSONBody(httptest.NewRecorder(), r, &body, true)
			} else {
				err = decodeSingleJSONBody(httptest.NewRecorder(), r, &body, true)
			}
			if (err == nil) != tc.valid {
				t.Fatalf("decode: %v", err)
			}
		})
	}
}
