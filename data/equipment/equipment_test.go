package equipmentdata

import (
	"bytes"
	"io/fs"
	"testing"
)

func TestBundledIconsAreSVG(t *testing.T) {
	names, err := fs.Glob(iconFiles, "icons/*.svg")
	if err != nil || len(names) < 80 {
		t.Fatalf("icons: %v, %d files", err, len(names))
	}
	for _, name := range names {
		data, _ := iconFiles.ReadFile(name)
		if !bytes.Contains(data, []byte("<svg")) || bytes.Contains(data, []byte("<script")) {
			t.Errorf("%s is not a clean SVG", name)
		}
	}
	if Icon("smokegrenade") == nil || Icon("not_an_icon") != nil {
		t.Error("Icon must find smokegrenade and nothing for an unknown name")
	}
}
