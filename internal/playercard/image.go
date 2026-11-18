package playercard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/storage"
)

// ImageRef names an uploaded overlay image by id and the hash its bytes had.
type ImageRef struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

// validSteamID reports whether id is a SteamID64, the only key a player image is kept under.
func validSteamID(id string) bool {
	return len(id) == 17 && strings.HasPrefix(id, "7656119") && strings.Trim(id, "0123456789") == ""
}

func imageKey(steamID string) string { return path.Join("player-images", steamID+".json") }

// treeDeleter is the optional storage capability that removes a replaced image.
type treeDeleter interface {
	DeleteTree(key string) error
}

// SaveImage remembers an uploaded overlay image for the player and removes the one it replaces;
// an empty ref forgets the image.
func SaveImage(files storage.Storage, steamID string, ref ImageRef) error {
	if !validSteamID(steamID) {
		return fmt.Errorf("invalid SteamID64 %q", steamID)
	}
	previous, found, _ := loadImage(files, steamID)
	body, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	if err := files.Put(imageKey(steamID), bytes.NewReader(body)); err != nil {
		return err
	}
	deleter, canDelete := files.(treeDeleter)
	if id, err := uuid.Parse(previous.ID); found && canDelete && err == nil && previous.ID != ref.ID {
		_ = deleter.DeleteTree(path.Dir(overlayassets.MediaKey(id)))
	}
	return nil
}

// loadImage returns the image remembered for the player, if any.
func loadImage(files storage.Storage, steamID string) (ImageRef, bool, error) {
	if !validSteamID(steamID) {
		return ImageRef{}, false, nil
	}
	r, err := files.Open(imageKey(steamID))
	if storage.IsNotExist(err) {
		return ImageRef{}, false, nil
	}
	if err != nil {
		return ImageRef{}, false, err
	}
	defer r.Close()
	var ref ImageRef
	if err := json.NewDecoder(io.LimitReader(r, 4096)).Decode(&ref); err != nil {
		return ImageRef{}, false, err
	}
	return ref, ref.ID != "", nil
}

// verifyImage checks that ref names a stored overlay image whose bytes still hash to it.
func verifyImage(ctx context.Context, files storage.Storage, ref ImageRef) error {
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return fmt.Errorf("invalid image id: %w", err)
	}
	image, err := overlayassets.Load(files, id)
	if err != nil || image.SHA256 != ref.SHA256 {
		return fmt.Errorf("image %s is not an uploaded overlay image", ref.ID)
	}
	return mediaassets.VerifyContent(ctx, files, overlayassets.MediaKey(id), ref.SHA256, overlayassets.MaxBytes)
}

func readImage(ctx context.Context, files storage.Storage, ref ImageRef) ([]byte, error) {
	if err := verifyImage(ctx, files, ref); err != nil {
		return nil, err
	}
	id, _ := uuid.Parse(ref.ID)
	r, err := files.Open(overlayassets.MediaKey(id))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, overlayassets.MaxBytes+1))
}
