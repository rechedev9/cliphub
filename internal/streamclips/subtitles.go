package streamclips

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/ffmpegfilter"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

type SubtitleRequest struct {
	JobID     uuid.UUID
	Summary   bool
	Stretches []library.VoiceSpan
}

type SubtitleTrack struct {
	Lines  []VoiceLine
	Hidden map[string][]uuid.UUID
	Notice string
}

type SubtitleVideo struct {
	ID     string
	Width  int
	Height int
	Clips  []ClipRange
}

type SubtitleScripts struct {
	ByClip      map[string]string
	Fingerprint string
}

func (p EditPlan) SubtitleVideos() []SubtitleVideo {
	layout, ok := VariantByName(p.Variant)
	if !ok {
		return nil
	}
	outputs := p.Outputs()
	videos := make([]SubtitleVideo, 0, len(outputs))
	for _, out := range outputs {
		videos = append(videos, SubtitleVideo{ID: out.ID, Width: layout.OutputWidth, Height: layout.FaceOutputHeight + layout.GameOutputHeight, Clips: out.Clips})
	}
	return videos
}

func SubtitleStretches(videos []SubtitleVideo) []library.VoiceSpan {
	var spans []library.VoiceSpan
	for _, v := range videos {
		for _, clip := range v.Clips {
			if !clip.SourceAudioMuted() {
				spans = append(spans, library.VoiceSpan{StartMS: int64(clip.StartSeconds * 1000), EndMS: int64(clip.EndSeconds * 1000)})
			}
		}
	}
	return spans
}

func NewSubtitleScripts(videos []SubtitleVideo, track SubtitleTrack) (SubtitleScripts, error) {
	out := SubtitleScripts{ByClip: map[string]string{}}
	sum := sha256.New()
	for _, v := range videos {
		video := subtitles.StreamVideo{Width: v.Width, Height: v.Height}
		for _, l := range track.Lines {
			if l.Discarded || slices.Contains(track.Hidden[v.ID], l.ID) {
				continue
			}
			line := subtitles.StreamLine{ID: l.ID.String(), Text: l.Text, StartSeconds: float64(l.StartMS) / 1000, EndSeconds: float64(l.EndMS) / 1000}
			for _, w := range l.Words {
				line.Words = append(line.Words, subtitles.Word{Text: w.Text, StartSeconds: float64(w.StartMS) / 1000, EndSeconds: float64(w.EndMS) / 1000})
			}
			video.Lines = append(video.Lines, line)
		}
		var drawn []ClipRange
		for _, clip := range v.Clips {
			if !clip.SourceAudioMuted() {
				drawn = append(drawn, clip)
				video.Parts = append(video.Parts, subtitles.StreamPart{StartSeconds: clip.StartSeconds, EndSeconds: clip.EndSeconds, Speed: clip.EffectiveSpeed()})
			}
		}
		scripts, err := subtitles.StreamASS(video)
		if err != nil {
			return SubtitleScripts{}, err
		}
		for i, script := range scripts {
			if script == "" {
				continue
			}
			out.ByClip[drawn[i].ID] = script
			sum.Write([]byte(v.ID + "\x00" + drawn[i].ID + "\x00" + script + "\x00"))
		}
	}
	if len(out.ByClip) > 0 {
		out.Fingerprint = hex.EncodeToString(sum.Sum(nil))
	}
	return out, nil
}

func (s SubtitleScripts) WriteFilters(dir string) (map[string]string, error) {
	filters := map[string]string{}
	if len(s.ByClip) == 0 {
		return filters, nil
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	for clip, script := range s.ByClip {
		path := filepath.Join(dir, clip+".ass")
		if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
			return nil, err
		}
		filters[clip] = "ass=filename=" + ffmpegfilter.Path(path) + ":fontsdir=" + ffmpegfilter.Path(fonts)
	}
	return filters, nil
}
