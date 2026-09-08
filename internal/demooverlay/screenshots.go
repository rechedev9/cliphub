package demooverlay

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"

	"github.com/rechedev9/cliphub/internal/overlayassets"
)

// ScreenshotHTML places the original screenshot pixels inside fixed areas.
// object-fit:contain preserves the full image, including names and statistics.
func ScreenshotHTML(s Screenshots, outro, previewGrey bool) ([]byte, error) {
	data := struct {
		Outro, PreviewGrey      bool
		Left, Right, Scoreboard template.URL
	}{Outro: outro, PreviewGrey: previewGrey}
	assetURL := func(file *ScreenshotFile) (template.URL, error) {
		if file == nil {
			return "", nil
		}
		stat, err := os.Stat(file.Path)
		if err != nil {
			return "", err
		}
		if stat.Size() > overlayassets.MaxBytes {
			return "", fmt.Errorf("Screenshot exceeds 10 MB")
		}
		body, err := os.ReadFile(file.Path)
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(body)
		if hex.EncodeToString(hash[:]) != file.SHA256 {
			return "", fmt.Errorf("Screenshot content differs from the approved image")
		}
		_, mime, err := overlayassets.Decode(body)
		if err != nil {
			return "", err
		}
		return template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(body)), nil // #nosec G203 -- decoded pixels, never HTML.
	}
	var err error
	if outro {
		data.Scoreboard, err = assetURL(s.Scoreboard)
	} else {
		if (s.Team1 == nil) != (s.Team2 == nil) {
			return nil, fmt.Errorf("Both team screenshots are required for the introduction")
		}
		data.Left, err = assetURL(s.Team1)
		if err == nil {
			data.Right, err = assetURL(s.Team2)
		}
	}
	if err != nil {
		return nil, err
	}
	raw, err := neonAssets.ReadFile("screenshots.html")
	if err != nil {
		return nil, err
	}
	t, err := template.New("screenshots").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
