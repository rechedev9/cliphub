package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/composition"
)

func fullDemoConcatListPath(short ShortEdit) string {
	return filepath.Join(filepath.Dir(short.Output), "concat-list.txt")
}

func writeFullDemoConcatList(short ShortEdit) error {
	if !isFullDemoNative(short.Preset, short.OutputFormat, len(short.Parts) > 0) {
		return nil
	}
	list, err := fullDemoConcatList(short)
	if err != nil {
		return err
	}
	path := fullDemoConcatListPath(short)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("full demo concat dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(list), 0o600); err != nil {
		return fmt.Errorf("write concat list: %w", err)
	}
	return nil
}

func fullDemoConcatList(short ShortEdit) (string, error) {
	if strings.TrimSpace(short.MusicPath) != "" {
		return "", fmt.Errorf("full demo concat does not mix a music bed")
	}
	var b strings.Builder
	b.WriteString("ffconcat version 1.0\n")
	for _, part := range short.Parts {
		if part.GapBeforeSeconds > 0.001 {
			return "", fmt.Errorf("full demo concat does not insert rhythm gaps (%s has %.3fs before it)", part.SegmentID, part.GapBeforeSeconds)
		}
		if strings.TrimSpace(part.Input) == "" {
			return "", fmt.Errorf("full demo concat part %s has no input", part.SegmentID)
		}
		b.WriteString(composition.ConcatFileLine(part.Input))
		fmt.Fprintf(&b, "outpoint %.6f\n", compilationPartDuration(short, part))
	}
	return b.String(), nil
}

func buildFullDemoCompilationCommand(ffmpegPath string, short ShortEdit) []string {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	command := []string{
		ffmpegPath,
		"-y",
		"-v", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", fullDemoConcatListPath(short),
	}
	for _, effect := range imageEffects(short.Effects) {
		command = append(command, "-i", effect.Path)
	}
	for _, track := range short.VoiceTracks {
		command = append(command, "-i", track)
	}
	command = append(command,
		"-filter_complex", fullDemoCompilationFilter(short),
		"-map", "[v]",
		"-map", "[a]",
	)
	command = appendVideoEncodeArgs(command, short)
	command = appendAudioCodecArgs(command)
	command = append(command, "-movflags", "+faststart")
	command = appendThreadArgs(command, short)
	return append(command, short.Output)
}

func fullDemoCompilationFilter(short ShortEdit) string {
	partShort := short
	partShort.Effects = nil
	partShort.Parts = nil
	clauses := []string{
		fmt.Sprintf("[0:v]%s[vgeom]", VideoFilter(partShort)),
	}
	clauses = appendCompilationProgramVideo(clauses, short, "vgeom", 1)
	clauses = append(clauses, "[0:a]aformat=channel_layouts=stereo,aresample=48000[ga0]")
	if mix := fullDemoVoiceMixFilter(short); mix != "" {
		clauses = append(clauses, mix)
	} else {
		clauses = append(clauses, "[ga0]anull[gamea]")
	}
	if short.AudioNormalize {
		clauses = append(clauses, "[gamea]loudnorm=I=-16:TP=-1.5:LRA=11[a]")
	} else {
		clauses = append(clauses, "[gamea]anull[a]")
	}
	return strings.Join(clauses, ";")
}

func fullDemoVoiceMixFilter(short ShortEdit) string {
	if len(short.VoiceTracks) == 0 || short.VoiceTickrate <= 0 {
		return ""
	}
	voiceInputStart := 1 + len(imageEffects(short.Effects))
	voiceVol := fmt.Sprintf("%.2f", voiceMixVolume(short))
	var clauses []string
	var trackLabels []string
	for t := range short.VoiceTracks {
		var labels []string
		for i, part := range short.Parts {
			partDuration := compilationPartDuration(short, part)
			start, end := voiceMixWindow(part, short.VoiceTickrate, partDuration)
			lab := fmt.Sprintf("vt%d_%d", t, i)
			if end <= start {
				clauses = append(clauses, fmt.Sprintf(
					"anullsrc=channel_layout=stereo:sample_rate=48000:d=%.6f[%s]",
					partDuration, lab,
				))
			} else {
				clauses = append(clauses, fmt.Sprintf(
					"[%d:a]atrim=start=%.6f:end=%.6f,asetpts=PTS-STARTPTS,aformat=channel_layouts=stereo,aresample=48000,volume=%s[%s]",
					voiceInputStart+t, start, end, voiceVol, lab,
				))
			}
			labels = append(labels, "["+lab+"]")
		}
		if len(labels) == 0 {
			continue
		}
		out := fmt.Sprintf("vcat%d", t)
		if len(labels) == 1 {
			clauses = append(clauses, fmt.Sprintf("%sanull[%s]", labels[0], out))
		} else {
			clauses = append(clauses, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[%s]", strings.Join(labels, ""), len(labels), out))
		}
		trackLabels = append(trackLabels, "["+out+"]")
	}
	if len(trackLabels) == 0 {
		return ""
	}
	voiceLab := "vmix"
	if len(trackLabels) == 1 {
		clauses = append(clauses, fmt.Sprintf("%sanull[%s]", trackLabels[0], voiceLab))
	} else {
		clauses = append(clauses, fmt.Sprintf("%samix=inputs=%d:duration=longest:dropout_transition=0:normalize=0[%s]",
			strings.Join(trackLabels, ""), len(trackLabels), voiceLab))
	}
	clauses = append(clauses, fmt.Sprintf("[ga0][%s]amix=inputs=2:duration=first:dropout_transition=0:normalize=0[gamea]", voiceLab))
	return strings.Join(clauses, ";")
}
