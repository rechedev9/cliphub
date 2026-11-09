package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// streamUpload is an uploaded stream source on its way to storage.
type streamUpload struct {
	file   *os.File
	key    string
	probe  streamclips.SourceProbe
	sha256 string
}

// storeStreamUpload stores an upload, first remuxing to MP4 a container the editor
// cannot open, such as an MPEG-TS recording renamed to .mp4. It returns what was stored.
func (h *Handlers) storeStreamUpload(ctx context.Context, up streamUpload) (streamUpload, error) {
	if !up.probe.NeedsRemux() {
		if _, err := up.file.Seek(0, io.SeekStart); err != nil {
			return up, fmt.Errorf("rewind stream upload: %w", err)
		}
		return up, h.storage.Put(up.key, up.file)
	}
	remuxedPath := up.file.Name() + ".remux.mp4"
	defer os.Remove(remuxedPath)
	if err := streamclips.RemuxToMP4(ctx, streamclips.RemuxOptions{FFmpeg: h.streamFFmpeg, Input: up.file.Name(), Output: remuxedPath}); err != nil {
		return up, err
	}
	probe, err := h.streamProber.Probe(ctx, remuxedPath)
	if err != nil {
		return up, fmt.Errorf("probe remuxed stream upload: %w", err)
	}
	if probe.NeedsRemux() {
		return up, fmt.Errorf("%w: still %s after remux", streamclips.ErrSourceNotPlayable, probe.Container)
	}
	remuxed, err := os.Open(remuxedPath)
	if err != nil {
		return up, err
	}
	defer remuxed.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, remuxed); err != nil {
		return up, fmt.Errorf("hash remuxed stream upload: %w", err)
	}
	if _, err := remuxed.Seek(0, io.SeekStart); err != nil {
		return up, fmt.Errorf("rewind remuxed stream upload: %w", err)
	}
	up.probe, up.sha256 = probe, hex.EncodeToString(hash.Sum(nil))
	return up, h.storage.Put(up.key, remuxed)
}

func writeStreamUploadError(w http.ResponseWriter, err error) {
	if errors.Is(err, streamclips.ErrSourceNotPlayable) {
		writeCodedError(w, http.StatusBadRequest, streamclips.AcquireCodeNotPlayable,
			"Ese vídeo no es un MP4 que Studio pueda abrir y no pudimos convertirlo. Expórtalo como MP4 e impórtalo de nuevo.")
		return
	}
	internalError(w, "store stream source", err)
}
