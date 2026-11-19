package teamvoice

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/library"
)

// bytesPerMS is 16 kHz mono s16le.
const bytesPerMS = SampleRate / 1000 * 2

// pcmFile is a track decoded once to raw PCM on disk, read by region.
type pcmFile struct {
	*os.File
	size int64
}

func decodeTrack(ctx context.Context, media Decoder, src, dir string) (*pcmFile, error) {
	if media == nil {
		return nil, ErrNoFFmpeg
	}
	dst := filepath.Join(dir, "track.pcm")
	if err := media.RawPCM(ctx, src, dst); err != nil {
		return nil, err
	}
	f, err := os.Open(dst) // #nosec G304 -- the file this run decoded.
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &pcmFile{File: f, size: info.Size()}, nil
}

func (p *pcmFile) durationMS() int64 { return p.size / bytesPerMS }

// writeWAV writes one stretch of the track as a 16 kHz mono PCM WAV.
func (p *pcmFile) writeWAV(path string, span library.VoiceSpan) error {
	from := min(p.size, span.StartMS*bytesPerMS)
	length := min(p.size, span.EndMS*bytesPerMS) - from
	out, err := os.Create(path) // #nosec G304 -- a generated path in this run's folder.
	if err != nil {
		return err
	}
	header := make([]byte, 44)
	copy(header, "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(36+length))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], SampleRate)
	binary.LittleEndian.PutUint32(header[28:], SampleRate*2)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(length))
	_, err = out.Write(header)
	if err == nil {
		_, err = io.Copy(out, io.NewSectionReader(p, from, length))
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}
