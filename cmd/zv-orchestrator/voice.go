package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// voiceDeps keeps team voice under <DataDir>/voice and the whisper.cpp install under
// <DataDir>/tools/whisper. ZV_WHISPER_CLI and ZV_WHISPER_MODEL point at a local build instead.
func voiceDeps(cfg config) httpapi.VoiceDeps {
	if cfg.FFmpegPath == "" {
		log.Printf("voice: no FFmpeg, team voice import is disabled")
		return httpapi.VoiceDeps{}
	}
	media := teamvoice.Media{FFmpeg: cfg.FFmpegPath, FFprobe: cfg.FFprobePath}
	return httpapi.VoiceDeps{
		Dir:   filepath.Join(cfg.DataDir, "voice"),
		Media: media,
		Transcriber: teamvoice.NewTranscriber(teamvoice.TranscriberOptions{
			Dir:   filepath.Join(cfg.DataDir, "tools", "whisper"),
			Media: media,
			CLI:   os.Getenv("ZV_WHISPER_CLI"),
			Model: os.Getenv("ZV_WHISPER_MODEL"),
		}),
	}
}
