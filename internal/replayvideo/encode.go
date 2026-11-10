package replayvideo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// maxFrameBytes bounds one JPEG frame: a 2560 px frame at top quality stays well under it.
const maxFrameBytes = 32 << 20

var (
	// ErrNotJPEG is a frame that does not decode as a JPEG.
	ErrNotJPEG = errors.New("replayvideo: a frame is not a JPEG")
	// ErrFrameSize is a frame whose size is not the plan's.
	ErrFrameSize = errors.New("replayvideo: a frame does not have the video's size")
	// ErrFrameRead is an upload that broke inside a frame, such as a client that went away.
	ErrFrameRead = errors.New("replayvideo: a frame could not be read")
)

// Sink takes a running encode's frames in order.
type Sink interface {
	Frame(src io.Reader) error
	// Finish waits for FFmpeg once every frame went in; voice says whether the MP4 has audio.
	Finish() (voice bool, err error)
	// Abort stops FFmpeg and removes what it wrote.
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
	if p.GraphFile == "" {
		p.GraphFile = p.Out + ".filter"
	}
	cmdLine, err := Args(p)
	if err != nil {
		return nil, err
	}
	run := &run{plan: p}
	if cmdLine.Graph != "" {
		if err := os.WriteFile(p.GraphFile, []byte(cmdLine.Graph), 0o600); err != nil {
			return nil, err
		}
		run.graphFile = p.GraphFile
	}
	ctx, cancel := context.WithCancel(ctx)
	run.cancel = cancel
	// #nosec G204 -- the configured FFmpeg receives generated local paths as argv values.
	run.cmd = exec.CommandContext(ctx, e.FFmpeg, cmdLine.Args...)
	run.voice = cmdLine.Voice
	run.cmd.Stderr = &run.stderr
	if run.stdin, err = run.cmd.StdinPipe(); err == nil {
		err = run.cmd.Start()
	}
	if err != nil {
		cancel()
		run.removeGraph()
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	return run, nil
}

type run struct {
	plan      Plan
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	cancel    context.CancelFunc
	stderr    bytes.Buffer
	voice     bool
	graphFile string
	once      sync.Once
	err       error
}

func (r *run) Frame(src io.Reader) error {
	raw, err := io.ReadAll(io.LimitReader(src, maxFrameBytes+1))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFrameRead, err)
	}
	if len(raw) > maxFrameBytes {
		return ErrNotJPEG
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return ErrNotJPEG
	}
	if cfg.Width != r.plan.Width || cfg.Height != r.plan.Height {
		return fmt.Errorf("%w: %dx%d, not %dx%d", ErrFrameSize, cfg.Width, cfg.Height, r.plan.Width, r.plan.Height)
	}
	if _, err := r.stdin.Write(raw); err != nil {
		if werr := r.wait(); werr != nil {
			return werr
		}
		return err
	}
	return nil
}

func (r *run) wait() error {
	r.once.Do(func() {
		_ = r.stdin.Close()
		if err := r.cmd.Wait(); err != nil {
			r.err = fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(r.stderr.String()))
		}
		r.cancel()
		r.removeGraph()
	})
	return r.err
}

func (r *run) removeGraph() {
	if r.graphFile != "" {
		_ = os.Remove(r.graphFile)
	}
}

func (r *run) Finish() (bool, error) {
	return r.voice, r.wait()
}

func (r *run) Abort() {
	r.cancel()
	_ = r.wait()
	_ = os.Remove(r.plan.Out)
}
