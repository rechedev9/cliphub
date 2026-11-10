package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/voicecomms"
	"github.com/rechedev9/cliphub/internal/voiceplugin"
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
		Dir:       filepath.Join(cfg.DataDir, "voice"),
		Media:     media,
		DemoVoice: demoVoiceExtractor{ffmpeg: cfg.FFmpegPath},
		Transcriber: teamvoice.NewTranscriber(teamvoice.TranscriberOptions{
			Dir:   filepath.Join(cfg.DataDir, "tools", "whisper"),
			Media: media,
			CLI:   os.Getenv("ZV_WHISPER_CLI"),
			Model: os.Getenv("ZV_WHISPER_MODEL"),
		}),
		// The plugin finds this folder from the profile; the desktop passes the web server CS2 posts to.
		Plugin: httpapi.VoicePluginDeps{Dir: voiceplugin.Dir(filepath.Join(cfg.DataDir, "voice-plugin")), WebOrigin: os.Getenv("ZV_WEB_ORIGIN"), Media: media},
	}
}

// demoVoiceExtractor writes a team's in-game voice as one Ogg per player, on the demo's clock.
type demoVoiceExtractor struct {
	ffmpeg string
}

func (e demoVoiceExtractor) ExtractDemoVoice(ctx context.Context, job httpapi.DemoVoiceJob) ([]voicecomms.Track, error) {
	res, err := voicecomms.ExtractFileWithContext(ctx, job.Demo, job.Target, job.Dir, e.ffmpeg)
	if err != nil {
		return nil, err
	}
	if res.Availability != voicecomms.Available {
		log.Printf("voice: demo %s has no playable voice for %s's team: %s", filepath.Base(job.Demo), job.Target, res.Availability)
		return nil, nil
	}
	return res.Index.Tracks, nil
}
