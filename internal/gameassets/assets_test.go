package gameassets

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// writeInstall lays out game\bin\win64\cs2.exe and a game\csgo\pak01_dir.vpk
// holding files inline, and returns the exe path.
func writeInstall(t *testing.T, files map[string][]byte) string {
	t.Helper()
	root := t.TempDir()
	var tree, data le
	for path, body := range files {
		dir, base := filepath.ToSlash(filepath.Dir(path)), filepath.Base(path)
		name, ext, _ := strings.Cut(base, ".")
		for _, s := range []string{ext, dir, name} {
			tree.WriteString(s + "\x00")
		}
		tree.u32(0)
		tree.u16(0)
		tree.u16(0x7fff)
		tree.u32(uint32(data.Len()))
		tree.u32(uint32(len(body)))
		tree.u16(0xffff)
		tree.WriteString("\x00\x00") // end of names, end of directories
		data.Write(body)
	}
	tree.WriteString("\x00")
	var pak le
	for _, v := range []uint32{0x55aa1234, 2, uint32(tree.Len()), uint32(data.Len()), 0, 0, 0} {
		pak.u32(v)
	}
	pak.Write(tree.Bytes())
	pak.Write(data.Bytes())
	exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	csgo := filepath.Join(root, "game", "csgo")
	for path, body := range map[string][]byte{exe: []byte("exe"), filepath.Join(csgo, "steam.inf"): []byte("ClientVersion=2000942\n"), filepath.Join(csgo, "pak01_dir.vpk"): pak.Bytes()} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return exe
}

func TestInstallRadarDecodesLayersToPNG(t *testing.T) {
	texture := buildResource(textureData(textureSpec{width: 2, height: 2, format: formatBGRA8888, mips: 1}), bgra2x2)
	exe := writeInstall(t, map[string][]byte{
		radarDir + "de_test_radar_psd.vtex_c":       texture,
		radarDir + "de_test_lower_radar_psd.vtex_c": texture,
		radarDir + "de_old_radar_tga.vtex_c":        texture,
	})
	install := Install{CS2Exe: exe}
	for _, c := range []struct{ name, layer string }{{"de_test", LayerUpper}, {"de_test", LayerLower}, {"de_old", LayerUpper}} {
		body, format, err := install.Radar(c.name, c.layer)
		if err != nil {
			t.Fatalf("%s %s: %v", c.name, c.layer, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(body))
		if err != nil || cfg.Width != 2 || format != "BGRA8888" {
			t.Errorf("%s %s: png %+v, %v, format %s", c.name, c.layer, cfg, err, format)
		}
	}
	if _, _, err := install.Radar("de_old", LayerLower); !errors.Is(err, ErrRadarNotFound) {
		t.Errorf("missing lower layer error = %v, want ErrRadarNotFound", err)
	}
	key, err := install.CacheKey("radar", "de_test_upper.png")
	if err != nil || !strings.HasPrefix(key, "gameassets/"+assetFormat+"/2000942-") || !strings.HasSuffix(key, "/radar/de_test_upper.png") {
		t.Errorf("CacheKey = %q, %v", key, err)
	}
}

func TestInstallIconServesSanitizedSVG(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" onload="x()"><path d="M0,0"/></svg>`
	exe := writeInstall(t, map[string][]byte{iconDir + "ak47.vsvg_c": buildResource(append([]byte{1, 2, 3, 4, 0, 0}, svg...), nil)})
	install := Install{CS2Exe: exe}
	body, contentType, err := install.Icon("ak47")
	if err != nil || contentType != ContentTypeSVG || strings.Contains(string(body), "onload") || !strings.Contains(string(body), `d="M0,0"`) {
		t.Fatalf("Icon = %q, %s, %v", body, contentType, err)
	}
	if _, _, err := install.Icon("frag"); !errors.Is(err, ErrIconNotFound) {
		t.Errorf("missing icon error = %v, want ErrIconNotFound", err)
	}
}

func TestInstallWithoutCS2(t *testing.T) {
	for _, exe := range []string{"", filepath.Join(t.TempDir(), "game", "bin", "win64", "cs2.exe")} {
		install := Install{CS2Exe: exe}
		if _, _, err := install.Radar("de_mirage", LayerUpper); !errors.Is(err, mapgeo.ErrCS2NotFound) {
			t.Errorf("Radar with exe %q: %v, want ErrCS2NotFound", exe, err)
		}
		if _, err := install.CacheKey("icon", "ak47.svg"); !errors.Is(err, mapgeo.ErrCS2NotFound) {
			t.Errorf("CacheKey with exe %q: %v, want ErrCS2NotFound", exe, err)
		}
	}
}
