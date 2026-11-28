package workers

import (
	"log"
	"sync"

	"github.com/rechedev9/cliphub/internal/recording"
)

// Capture and render can encode with different ffmpeg binaries: HLAE's
// mirv_streams uses the ffmpeg HLAE resolves next to HLAE.exe (bundled bin or
// ffmpeg.ini, which the recorder points at Studio's ffmpeg when it has one),
// while zv-editor uses the orchestrator-resolved ffmpeg. Each side probes its
// own binary once; tool paths do not change mid-process.
var studioEncoders = struct {
	mu            sync.Mutex
	captureProbed bool
	capture       string
	renderProbed  bool
	render        string
}{}

// studioCaptureEncoder decides the HLAE stream encoder for recorder launches.
// It probes the ffmpeg HLAE will actually invoke for the configured HLAE.exe
// and ffmpeg, and returns EncoderNVENC only when that binary can open
// h264_nvenc here; otherwise capture stays on the default software encoder.
func studioCaptureEncoder(hlaePath, ffmpegPath string) string {
	studioEncoders.mu.Lock()
	defer studioEncoders.mu.Unlock()
	if !studioEncoders.captureProbed {
		studioEncoders.captureProbed = true
		ffmpegPath := recording.HLAEStreamFFmpeg(hlaePath, ffmpegPath)
		if err := recording.CheckEncoderUsable(ffmpegPath, recording.EncoderNVENC); err != nil {
			log.Printf("workers: studio capture NVENC unavailable: %v", err)
		} else {
			studioEncoders.capture = recording.EncoderNVENC
			log.Printf("workers: studio NVENC enabled for capture via %s", ffmpegPath)
		}
	}
	return studioEncoders.capture
}

// studioRenderVideoEncoder decides the zv-editor video encoder using the
// orchestrator-resolved ffmpeg, the binary the editor actually invokes.
func studioRenderVideoEncoder(ffmpegPath string) string {
	studioEncoders.mu.Lock()
	defer studioEncoders.mu.Unlock()
	if !studioEncoders.renderProbed {
		studioEncoders.renderProbed = true
		if ffmpegPath == "" {
			ffmpegPath = recording.FindFFmpeg()
		}
		studioEncoders.render = firstUsableRenderEncoder(func(encoder string) error {
			return recording.CheckEncoderUsable(ffmpegPath, encoder)
		})
		if studioEncoders.render != "" {
			log.Printf("workers: studio %s enabled for render via %s", studioEncoders.render, ffmpegPath)
		}
	}
	return studioEncoders.render
}

func firstUsableRenderEncoder(usable func(encoder string) error) string {
	for _, encoder := range []string{recording.EncoderNVENC, recording.EncoderAMF} {
		err := usable(encoder)
		if err == nil {
			return encoder
		}
		log.Printf("workers: studio render %s unavailable: %v", encoder, err)
	}
	return ""
}
