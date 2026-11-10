package main

import (
	"log"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/replayvideo"
)

// replayVideoDeps enables the Replay's MP4 export; it needs FFmpeg.
func replayVideoDeps(cfg config) httpapi.ReplayVideoDeps {
	if cfg.FFmpegPath == "" {
		log.Printf("replay video: no FFmpeg, the Replay's MP4 export is disabled")
		return httpapi.ReplayVideoDeps{}
	}
	return httpapi.ReplayVideoDeps{Encoder: replayvideo.Encoder{FFmpeg: cfg.FFmpegPath}, Dir: filepath.Join(cfg.DataDir, "replay-video")}
}
