// Package overlayassets stores immutable, decoded screenshots for Full Demo.
package overlayassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/storage"
)

const MaxBytes = 10 << 20

type Asset struct {
	ID          uuid.UUID `json:"id"`
	SHA256      string    `json:"sha256"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
}

func MediaKey(id uuid.UUID) string    { return path.Join("overlay-images", id.String(), "image") }
func metadataKey(id uuid.UUID) string { return path.Join("overlay-images", id.String(), "asset.json") }

func Decode(data []byte) (image.Image, string, error) {
	if len(data) == 0 || len(data) > MaxBytes {
		return nil, "", fmt.Errorf("La captura debe ocupar entre 1 byte y 10 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, "", fmt.Errorf("Selecciona una captura PNG o JPG válida")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 32_000_000 {
		return nil, "", fmt.Errorf("La captura supera los límites de 8192 píxeles por lado o 32 megapíxeles")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("No se pudo decodificar la captura: %w", err)
	}
	return decoded, "image/" + format, nil
}

func Store(ctx context.Context, store storage.Storage, input io.Reader, fileName string) (Asset, error) {
	body, err := io.ReadAll(io.LimitReader(input, MaxBytes+1))
	if err != nil {
		return Asset{}, err
	}
	decoded, contentType, err := Decode(body)
	if err != nil {
		return Asset{}, err
	}
	if err := ctx.Err(); err != nil {
		return Asset{}, err
	}
	hash := sha256.Sum256(body)
	a := Asset{ID: uuid.New(), SHA256: hex.EncodeToString(hash[:]), FileName: mediaassets.SanitizeFileName(fileName), ContentType: contentType, Width: decoded.Bounds().Dx(), Height: decoded.Bounds().Dy()}
	if err := store.Put(MediaKey(a.ID), bytes.NewReader(body)); err != nil {
		return Asset{}, err
	}
	metadata, err := json.Marshal(a)
	if err != nil {
		return Asset{}, err
	}
	// Publish metadata last: incomplete uploads cannot become selectable.
	if err := store.Put(metadataKey(a.ID), bytes.NewReader(metadata)); err != nil {
		return Asset{}, err
	}
	return a, nil
}

func Load(store storage.Storage, id uuid.UUID) (Asset, error) {
	r, err := store.Open(metadataKey(id))
	if err != nil {
		return Asset{}, err
	}
	defer r.Close()
	var a Asset
	dec := json.NewDecoder(io.LimitReader(r, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return Asset{}, err
	}
	if dec.Decode(new(any)) != io.EOF || a.ID != id || id == uuid.Nil || len(a.SHA256) != 64 || strings.Trim(a.SHA256, "0123456789abcdef") != "" || (a.ContentType != "image/png" && a.ContentType != "image/jpeg") || a.Width <= 0 || a.Height <= 0 || a.Width > 8192 || a.Height > 8192 || int64(a.Width)*int64(a.Height) > 32_000_000 {
		return Asset{}, fmt.Errorf("Invalid screenshot metadata")
	}
	return a, nil
}
