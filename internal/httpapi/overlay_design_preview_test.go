package httpapi

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestOverlayDesignPreviewDrawsEachDesignAsAnImage(t *testing.T) {
	env := newLibraryTestEnv(t)
	neutral := demooverlay.DesignPalette(recapplan.OverlayOptions{Theme: recapplan.OverlayThemeHUD})
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"design=neon", []string{"#e69bff", "Players", "donk"}},
		{"design=neon&outro=1", []string{"#d99aff", "Match results"}},
		{"design=hud&base=prism", []string{"#AC91FF", "#68EDBE", "Players"}},
		{"design=hud&base=prism&ct=112233&t=445566&background=000000&radius=4&outro=1", []string{"#112233", "#445566", "4px", "Match results"}},
		{"design=hud", []string{"#" + neutral.Background}},
	} {
		rw := env.do(http.MethodGet, "/api/full-demo/overlay-designs/preview?"+tc.query, nil, "")
		body := rw.Body.String()
		if rw.Code != http.StatusOK || rw.Header().Get("Content-Type") != "image/svg+xml" {
			t.Fatalf("%s = %d %s, %.200s", tc.query, rw.Code, rw.Header().Get("Content-Type"), body)
		}
		decoder := xml.NewDecoder(strings.NewReader(body))
		for {
			if _, err := decoder.Token(); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("%s is not an image the browser can parse: %v", tc.query, err)
			}
		}
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not show %q", tc.query, want)
			}
		}
	}
	for _, bad := range []string{"design=orange", "design=hud&base=nope", "design=hud&base=arena&ct=zz&t=445566&background=000000"} {
		if rw := env.do(http.MethodGet, "/api/full-demo/overlay-designs/preview?"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", bad, rw.Code)
		}
	}
}
