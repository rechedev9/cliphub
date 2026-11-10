package replayvideo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// ErrNotJPEG is a frame that does not start like a JPEG.
var ErrNotJPEG = errors.New("replayvideo: a frame is not a JPEG")

// Sink takes a running encode's frames in order.
type Sink interface {
	Frame(src io.Reader) error
	// Finish waits for FFmpeg once every frame went in; voice says whether the MP4 has audio.
	Finish() (voice bool, err error)
	Abort()
}

// Encoder runs the FFmpeg Studio ships.
type Encoder struct {
	FFmpeg string
}

// Start runs FFmpeg for a plan, waiting for its frames on stdin.
func (e Encoder) Start(ctx context.Context, p Plan) (Sink, error) {
	if e.FFmpeg == "" {
		return nil, errors.New("replayvideo: FFmpeg is not configured")
	}
	args, voice, err := Args(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	// #nosec G204 -- the configured FFmpeg receives generated local paths as argv values.
	cmd := exec.CommandContext(ctx, e.FFmpeg, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	run := &run{cmd: cmd, stdin: stdin, cancel: cancel, voice: voice}
	cmd.Stderr = &run.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	return run, nil
}

type run struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc
	stderr bytes.Buffer
	voice  bool
	once   sync.Once
	err    error
}

func (r *run) Frame(src io.Reader) error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(src, head); err != nil || head[0] != 0xff || head[1] != 0xd8 {
		return ErrNotJPEG
	}
	if _, err := r.stdin.Write(head); err != nil {
		return r.failed(err)
	}
	if _, err := io.Copy(r.stdin, src); err != nil {
		return r.failed(err)
	}
	return nil
}

// failed is a write FFmpeg refused: its own error says why.
func (r *run) failed(err error) error {
	if werr := r.wait(); werr != nil {
		return werr
	}
	return err
}

func (r *run) wait() error {
	r.once.Do(func() {
		_ = r.stdin.Close()
		if err := r.cmd.Wait(); err != nil {
			r.err = fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(r.stderr.String()))
		}
		r.cancel()
	})
	return r.err
}

func (r *run) Finish() (bool, error) {
	return r.voice, r.wait()
}

func (r *run) Abort() {
	r.cancel()
	_ = r.wait()
}
