package mediaassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/storage"
)

// Provenance records a user's permission declaration, not an inferred license.
type Provenance struct {
	SchemaVersion string `json:"schema_version"`
	AssetSHA256   string `json:"asset_sha256"`
	Title         string `json:"title"`
	Creator       string `json:"creator"`
	SourceURL     string `json:"source_url"`
	Permission    string `json:"permission"`
	Attribution   string `json:"attribution"`
}

func (p Provenance) Validate() error {
	if p.SchemaVersion != "1.0" || !sha256HexPattern.MatchString(p.AssetSHA256) {
		return fmt.Errorf("invalid provenance version or asset hash")
	}
	for _, field := range []struct {
		name, value string
		max         int
	}{{"title", p.Title, 200}, {"creator", p.Creator, 200}, {"source_url", p.SourceURL, 2000}, {"permission", p.Permission, 4000}} {
		if strings.TrimSpace(field.value) == "" || len(field.value) > field.max {
			return fmt.Errorf("asset %s is required and must fit its size limit", field.name)
		}
	}
	if len(p.Attribution) > 4000 {
		return fmt.Errorf("asset attribution exceeds size limit")
	}
	u, err := url.Parse(p.SourceURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "local") || ((u.Scheme == "https" || u.Scheme == "http") && (u.Host == "" || u.User != nil)) {
		return fmt.Errorf("asset source must be an HTTP(S) source or explicit local declaration")
	}
	return nil
}

func StoreProvenance(store storage.Storage, id uuid.UUID, p Provenance) error {
	if err := p.Validate(); err != nil {
		return err
	}
	key := artifacts.MediaAssetProvenanceKey(id)
	if prior, ok, err := LoadProvenance(store, id); err != nil {
		return err
	} else if ok {
		if prior != p {
			return fmt.Errorf("asset provenance is immutable; import a new asset declaration")
		}
		return nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return store.Put(key, bytes.NewReader(b))
}

func LoadProvenance(store storage.Storage, id uuid.UUID) (Provenance, bool, error) {
	r, err := store.Open(artifacts.MediaAssetProvenanceKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return Provenance{}, false, nil
		}
		return Provenance{}, false, err
	}
	defer r.Close()
	var p Provenance
	dec := json.NewDecoder(io.LimitReader(r, 16385))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, false, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return p, false, fmt.Errorf("provenance must contain one JSON value")
	}
	if err := p.Validate(); err != nil {
		return p, false, err
	}
	return p, true, nil
}

// VerifyContent hashes the bytes actually served by storage, with cancellation
// and a hard bound. File names and historic probe metadata cannot prove identity.
func VerifyContent(ctx context.Context, store storage.Storage, key, wantHash string, maxBytes int64) error {
	r, err := store.Open(key)
	if err != nil {
		return fmt.Errorf("open asset content: %w", err)
	}
	defer r.Close()
	h := sha256.New()
	buf := make([]byte, 128<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := r.Read(buf)
		total += int64(n)
		if total > maxBytes {
			return fmt.Errorf("asset exceeds resource limit")
		}
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read asset content: %w", readErr)
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != wantHash {
		return fmt.Errorf("asset content hash changed")
	}
	return nil
}
