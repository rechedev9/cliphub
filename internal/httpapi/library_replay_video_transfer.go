package httpapi

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/replayvideo"
)

type replayVideoEncode struct {
	r       *http.Request
	reader  *multipart.Reader
	plan    replayvideo.Plan
	encoder ReplayVideoEncoder
}

// encodeReplayVideo feeds the "frame" parts to the encoder as they arrive and waits for the MP4.
// A broken upload, such as a client that went away, aborts the encode.
func encodeReplayVideo(e replayVideoEncode) (replayVideoResult, error) {
	sink, err := e.encoder.Start(e.r.Context(), e.plan)
	if err != nil {
		return replayVideoResult{}, err
	}
	want, got := e.plan.Frames(), 0
	fail := func(err error) (replayVideoResult, error) {
		sink.Abort()
		return replayVideoResult{}, err
	}
	for {
		part, err := e.reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(invalid("reading multipart body: " + err.Error()))
		}
		if part.FormName() != "frame" {
			_ = part.Close()
			continue
		}
		if got == want {
			_ = part.Close()
			return fail(invalid(fmt.Sprintf("the spec takes %d frames", want)))
		}
		err = sink.Frame(part)
		_ = part.Close()
		switch {
		case errors.Is(err, replayvideo.ErrNotJPEG):
			return fail(invalid(fmt.Sprintf("frame %d is not a JPEG", got+1)))
		case errors.Is(err, replayvideo.ErrFrameSize), errors.Is(err, replayvideo.ErrFrameRead):
			return fail(invalid(fmt.Sprintf("frame %d: %v", got+1, err)))
		case err != nil:
			return fail(err)
		}
		got++
	}
	if got != want {
		return fail(invalid(fmt.Sprintf("got %d frames, the spec takes %d", got, want)))
	}
	voice, err := sink.Finish()
	if err != nil {
		return replayVideoResult{}, err
	}
	info, err := os.Stat(e.plan.Out)
	if err != nil {
		return replayVideoResult{}, err
	}
	return replayVideoResult{Bytes: info.Size(), Frames: want, Seconds: e.plan.Seconds(), Voice: voice}, nil
}

// countingWriter remembers the status and how many body bytes went out.
type countingWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (c *countingWriter) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *countingWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	n, err := c.ResponseWriter.Write(b)
	c.written += int64(n)
	return n, err
}

// GetLibraryReplayVideo handles GET /api/library/replay-videos/{video}: the encoded MP4. A whole
// GET that wrote every byte ends its life; a HEAD, a range, a 304 or a broken transfer keep it.
func (h *Handlers) GetLibraryReplayVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := libraryUUIDParam(w, r, "video")
	if !ok {
		return
	}
	if h.replayVideo == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeReplayVideoUnavailable, "the Replay video needs FFmpeg, which this Studio did not start with")
		return
	}
	path := filepath.Join(h.replayVideo.Dir, id.String()+".mp4")
	f, err := os.Open(path) // #nosec G304 -- the name is a parsed UUID inside Studio's own folder.
	if err != nil {
		writeError(w, http.StatusNotFound, "no such replay video; export it again")
		return
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		internalError(w, "stat replay video", err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	cw := &countingWriter{ResponseWriter: w}
	http.ServeContent(cw, r, "", info.ModTime(), f)
	_ = f.Close()
	if r.Method == http.MethodGet && r.Header.Get("Range") == "" && cw.status == http.StatusOK && cw.written == info.Size() {
		_ = os.Remove(path)
	}
}
