package mediafont

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

const HUDFamily = "Barlow Semi Condensed"

//go:embed barlow/*.ttf
var hudFonts embed.FS

// HUDFace describes one embedded face. The weight is shared by measurement,
// SVG and libass; no system font installation is required.
type HUDFace struct {
	// Full name (OpenType name ID 4) disambiguates weights on DirectWrite.
	// Using their shared ID 1 family can resolve every weight to Bold.
	Family string
	Weight int
	File   string
	SHA256 string
	Data   []byte
}

func HUDFaces() ([]HUDFace, error) {
	faces := []HUDFace{
		{Family: HUDFamily + " Medium", Weight: 500, File: "BarlowSemiCondensed-Medium.ttf", SHA256: "b5961bc7009f4e08ffce90931ddca85545357b8f260f2ed1b37c3241ce71c2db"},
		{Family: HUDFamily + " SemiBold", Weight: 600, File: "BarlowSemiCondensed-SemiBold.ttf", SHA256: "76b0db666060f767d7f9707d1fe8f668eeb114c0a2eab90fd657e454ab6b171c"},
		{Family: HUDFamily + " Bold", Weight: 700, File: "BarlowSemiCondensed-Bold.ttf", SHA256: "0fb7401b4bb43e284bebf66c69fb42b5462380cbfd352a69fa0fb3f0a774c31a"},
	}
	for i := range faces {
		body, err := hudFonts.ReadFile("barlow/" + faces[i].File)
		if err != nil {
			return nil, err
		}
		faces[i].Data = body
	}
	return append(faces, HUDFace{Family: FamilyName, Weight: 800, File: FileName, SHA256: EmbeddedSHA256, Data: montserratExtraBold}), nil
}

// MaterializeHUD returns a directory containing only the HUD faces and its
// bundled Cyrillic fallback. Other generated media retain their existing font.
func MaterializeHUD() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil || root == "" {
		root = os.TempDir()
	}
	if root == "" {
		return "", fmt.Errorf("materialize HUD fonts: no cache or temporary directory")
	}
	dir := filepath.Join(root, "ClipHub", "fonts", "hud-barlow-dc2940e-v1")
	faces, err := HUDFaces()
	if err != nil {
		return "", err
	}
	materializeMu.Lock()
	defer materializeMu.Unlock()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	for _, face := range faces {
		if err := materializeFile(dir, face.File, face.SHA256, face.Data); err != nil {
			return "", err
		}
	}
	return dir, nil
}
