package voicecomms

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/storage"
)

type StoredExtraction struct {
	Result      ExtractionResult  `json:"result"`
	IndexKey    string            `json:"index_key"`
	IndexHash   string            `json:"index_hash"`
	TrackHashes map[string]string `json:"track_hashes"`
}

var extractionLocks [64]sync.Mutex

// EnsureStored reuses one extraction per immutable input and extractor policy.
// Its index contains storage keys; temporary paths never enter a saved plan.
func EnsureStored(ctx context.Context, store storage.Storage, id uuid.UUID, demoKey, demoHash, target, ffmpeg string) (StoredExtraction, error) {
	h := sha256.Sum256([]byte(demoHash + "\n" + target + "\n" + ExtractorVersion))
	lock := &extractionLocks[h[0]%64]
	lock.Lock()
	defer lock.Unlock()
	if err := mediaassets.VerifyContent(ctx, store, demoKey, demoHash, 8<<30); err != nil {
		return StoredExtraction{}, fmt.Errorf("verify voice demo input: %w", err)
	}
	prefix, err := artifacts.FullDemoVoicePrefix(id, hex.EncodeToString(h[:]))
	if err != nil {
		return StoredExtraction{}, err
	}
	key := path.Join(prefix, "extraction.json")
	if r, err := store.Open(key); err == nil {
		var cached StoredExtraction
		decodeErr := json.NewDecoder(io.LimitReader(r, 8<<20)).Decode(&cached)
		closeErr := r.Close()
		if decodeErr != nil {
			return cached, fmt.Errorf("read cached voice index: %w", decodeErr)
		}
		if closeErr != nil {
			return cached, closeErr
		}
		if err := validateStored(ctx, store, cached); err != nil {
			return cached, err
		}
		return cached, nil
	} else if !storage.IsNotExist(err) {
		return StoredExtraction{}, err
	}
	dir, err := os.MkdirTemp("", "cliphub-full-demo-voice-")
	if err != nil {
		return StoredExtraction{}, err
	}
	defer os.RemoveAll(dir)
	r, err := store.Open(demoKey)
	if err != nil {
		return StoredExtraction{}, err
	}
	defer r.Close()
	demoPath := filepath.Join(dir, "source.dem")
	f, err := os.OpenFile(demoPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return StoredExtraction{}, err
	}
	copyErr := copyVoiceInput(ctx, f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return StoredExtraction{}, copyErr
	}
	if closeErr != nil {
		return StoredExtraction{}, closeErr
	}
	result, err := ExtractFileWithContext(ctx, demoPath, target, dir, ffmpeg)
	if err != nil {
		return StoredExtraction{Result: result}, err
	}
	result.Report.Demo = demoKey
	result.Index.Demo = demoKey
	stored := StoredExtraction{Result: result, IndexKey: path.Join(prefix, "voice-index.json"), TrackHashes: map[string]string{}}
	for i, track := range result.Index.Tracks {
		if err := ctx.Err(); err != nil {
			return stored, err
		}
		trackKey := path.Join(prefix, track.SteamID64+".ogg")
		f, err := os.Open(track.Path)
		if err != nil {
			return stored, err
		}
		digest := sha256.New()
		err = store.Put(trackKey, io.TeeReader(f, digest))
		closeErr := f.Close()
		if err != nil {
			return stored, err
		}
		if closeErr != nil {
			return stored, closeErr
		}
		stored.TrackHashes[trackKey] = hex.EncodeToString(digest.Sum(nil))
		stored.Result.Index.Tracks[i].Path = trackKey
	}
	b, err := json.Marshal(stored.Result.Index)
	if err != nil {
		return stored, err
	}
	indexDigest := sha256.Sum256(b)
	stored.IndexHash = hex.EncodeToString(indexDigest[:])
	if err := store.Put(stored.IndexKey, bytes.NewReader(b)); err != nil {
		return stored, err
	}
	b, err = json.Marshal(stored)
	if err != nil {
		return stored, err
	}
	if err := store.Put(key, bytes.NewReader(b)); err != nil {
		return stored, err
	}
	return stored, nil
}

func validateStored(ctx context.Context, store storage.Storage, cached StoredExtraction) error {
	if cached.Result.ExtractorVersion != ExtractorVersion {
		return fmt.Errorf("incompatible cached voice extractor")
	}
	if err := mediaassets.VerifyContent(ctx, store, cached.IndexKey, cached.IndexHash, 8<<20); err != nil {
		return err
	}
	r, err := store.Open(cached.IndexKey)
	if err != nil {
		return err
	}
	var actual Index
	decoder := json.NewDecoder(io.LimitReader(r, 8<<20))
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(&actual)
	if decodeErr == nil && decoder.Decode(new(any)) != io.EOF {
		decodeErr = fmt.Errorf("trailing cached voice index data")
	}
	closeErr := r.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !reflect.DeepEqual(actual, cached.Result.Index) {
		return fmt.Errorf("cached voice metadata differs from the verified index")
	}
	for _, track := range cached.Result.Index.Tracks {
		hash, ok := cached.TrackHashes[track.Path]
		if !ok {
			return fmt.Errorf("voice track lacks content hash")
		}
		if err := mediaassets.VerifyContent(ctx, store, track.Path, hash, 1<<30); err != nil {
			return err
		}
	}
	return nil
}

func copyVoiceInput(ctx context.Context, dst io.Writer, src io.Reader) error {
	buf := make([]byte, 128<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := src.Read(buf)
		if n > 0 {
			if count, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			} else if count != n {
				return io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
