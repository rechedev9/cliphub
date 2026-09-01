package workers

import (
	"log"
	"sync"

	"github.com/rechedev9/cliphub/internal/recording"
)

// Capture and render encode with different ffmpeg binaries: HLAE's
// mirv_streams uses the ffmpeg HLAE resolves next to HLAE.exe (bundled bin or
// ffmpeg.ini), while zv-editor uses the orchestrator-resolved ffmpeg. Each
// side probes its own binary once; tool paths do not change mid-process.
var studioEncoders = struct {
	mu            sync.Mutex
	captureProbed bool
	capture       string
	renderProbed  bool
	render        string
}{}

// studioCaptureEncoder decides the HLAE stream encoder for recorder launches.
// It probes the ffmpeg HLAE will actually invoke for the configured HLAE.exe
// and returns EncoderNVENC only when that binary advertises h264_nvenc;
// otherwise capture stays on the default software encoder.
func studioCaptureEncoder(hlaePath string) string {
	studioEncoders.mu.Lock()
	defer studioEncoders.mu.Unlock()
	if !studioEncoders.captureProbed {
		studioEncoders.captureProbed = true
		ffmpegPath := recording.HLAEStreamFFmpeg(hlaePath)
		if err := recording.CheckEncoderSupported(ffmpegPath, recording.EncoderNVENC); err != nil {
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
		if err := recording.CheckEncoderSupported(ffmpegPath, recording.EncoderNVENC); err != nil {
			log.Printf("workers: studio render NVENC unavailable: %v", err)
		} else {
			studioEncoders.render = recording.EncoderNVENC
			log.Printf("workers: studio NVENC enabled for render via %s", ffmpegPath)
		}
	}
	return studioEncoders.render
}
