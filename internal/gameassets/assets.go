// Package gameassets reads CS2's own radar images and equipment icons from the
// user's game install at runtime, so Studio never ships Valve art: radars are
// compiled textures decoded to PNG, icons are compiled SVGs served sanitized.
package gameassets

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// assetFormat versions the cache; bump it when the decoders change output.
const assetFormat = "v1"

// Radar layers: the main overview and the lower floor of two-level maps.
const (
	LayerUpper = "upper"
	LayerLower = "lower"
)

// Content types Icon returns.
const (
	ContentTypeSVG = "image/svg+xml"
	ContentTypePNG = "image/png"
)

var (
	// ErrRadarNotFound reports that the install has no radar for the map and layer.
	ErrRadarNotFound = errors.New("radar not found in the cs2 install")
	// ErrIconNotFound reports that the install has no icon with that name.
	ErrIconNotFound = errors.New("icon not found in the cs2 install")
)

const (
	radarDir = "panorama/images/overheadmaps/"
	iconDir  = "panorama/images/icons/equipment/"
)

// Install reads assets from the CS2 install of CS2Exe (game\bin\win64\cs2.exe).
type Install struct {
	CS2Exe string
}

func (i Install) pakPath() string {
	return filepath.Join(mapgeo.CSGODir(i.CS2Exe), "pak01_dir.vpk")
}

func (i Install) locate() (os.FileInfo, error) {
	if i.CS2Exe == "" {
		return nil, mapgeo.ErrCS2NotFound
	}
	info, err := os.Stat(i.pakPath())
	if err != nil {
		return nil, mapgeo.ErrCS2NotFound
	}
	return info, nil
}

// CacheKey names a cached asset file for this game build. The package's size
// and modification time change with every update even when steam.inf does not.
func (i Install) CacheKey(kind, file string) (string, error) {
	info, err := i.locate()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("gameassets/%s/%s-%d-%d/%s/%s", assetFormat,
		mapgeo.ClientVersion(mapgeo.CSGODir(i.CS2Exe)), info.Size(), info.ModTime().Unix(), kind, file), nil
}

func (i Install) open() (*mapgeo.VPK, error) {
	if _, err := i.locate(); err != nil {
		return nil, err
	}
	return mapgeo.OpenVPK(i.pakPath())
}

// radarPaths are the package paths tried for a map's radar, in order.
func radarPaths(mapName, layer string) []string {
	base := radarDir + mapName
	if layer == LayerLower {
		base += "_lower"
	}
	return []string{base + "_radar_psd.vtex_c", base + "_radar_tga.vtex_c"}
}

// Radar decodes the map's overview radar for a layer and encodes it as PNG.
// It also returns the texture's format name.
func (i Install) Radar(mapName, layer string) ([]byte, string, error) {
	pack, err := i.open()
	if err != nil {
		return nil, "", err
	}
	defer pack.Close()
	for _, path := range radarPaths(mapName, layer) {
		data, err := pack.ReadFile(path)
		if errors.Is(err, mapgeo.ErrNotInVPK) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return encodeTexture(data)
	}
	return nil, "", ErrRadarNotFound
}

func encodeTexture(resource []byte) ([]byte, string, error) {
	img, format, err := DecodeTexture(resource)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), FormatName(format), nil
}

// Icon returns an equipment icon: the sanitized SVG when CS2 ships a vector
// graphic, or a PNG decoded from a texture otherwise.
func (i Install) Icon(name string) ([]byte, string, error) {
	pack, err := i.open()
	if err != nil {
		return nil, "", err
	}
	defer pack.Close()
	data, err := pack.ReadFile(iconDir + name + ".vsvg_c")
	if err == nil {
		svg, err := ExtractSVG(data)
		if err != nil {
			return nil, "", err
		}
		clean, err := SanitizeSVG(svg)
		return clean, ContentTypeSVG, err
	}
	if !errors.Is(err, mapgeo.ErrNotInVPK) {
		return nil, "", err
	}
	data, err = pack.ReadFile(iconDir + name + ".vtex_c")
	if errors.Is(err, mapgeo.ErrNotInVPK) {
		return nil, "", ErrIconNotFound
	}
	if err != nil {
		return nil, "", err
	}
	body, _, err := encodeTexture(data)
	return body, ContentTypePNG, err
}
