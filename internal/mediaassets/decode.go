package mediaassets

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// VerifyDecoded accepts self-contained local media, not playlists or network
// sources. Probe metadata alone cannot establish that a selected asset decodes.
func VerifyDecoded(ctx context.Context, ffmpeg, file string) error {
	if ffmpeg == "" {
		return fmt.Errorf("FFmpeg is required to validate Full Demo media")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	// #nosec G204 -- configured executable and an owned local file passed as argv.
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error", "-xerror", "-protocol_whitelist", "file,pipe",
		"-format_whitelist", "mov,matroska,webm,mp3,wav,ogg,flac,aac,avi", "-i", file,
		"-map", "0:v:0?", "-map", "0:a:0?", "-f", "null", "-")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("asset decoding failed: %w", err)
	}
	return nil
}
