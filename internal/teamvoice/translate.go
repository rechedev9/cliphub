package teamvoice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/library"
)

// translatePadMS is the air kept around a phrase so whisper hears its first and last words.
const translatePadMS = 150

// Translator runs whisper.cpp in translate mode over the stretches already transcribed.
type Translator struct{ *tool }

func NewTranslator(opts TranscriberOptions) *Translator {
	return &Translator{newTool(opts, PinnedTranslator, "La traducción")}
}

// TranslateJob is one track and the phrases to translate, in audio time.
type TranslateJob struct {
	Audio    string
	Spans    []library.VoiceSpan
	Progress func(share float64)
}

// Translate returns the English of each span, in order; a span whisper hears nothing in is empty.
func (t *Translator) Translate(ctx context.Context, job TranslateJob) ([]string, error) {
	cli, model, err := t.ready()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(job.Spans))
	if len(job.Spans) == 0 {
		return out, nil
	}
	dir, err := os.MkdirTemp("", "cliphub-translate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	audio, err := decodeTrack(ctx, t.opts.Media, job.Audio, dir)
	if err != nil {
		return nil, err
	}
	defer audio.Close()
	for from := 0; from < len(job.Spans); from += regionBatch {
		to := min(len(job.Spans), from+regionBatch)
		args := []string{"-m", model, "-tr", "-l", "auto", "-t", threads(), "-oj", "-np"}
		for i := from; i < to; i++ {
			base := filepath.Join(dir, fmt.Sprintf("s%05d", i))
			span := library.VoiceSpan{StartMS: max(0, job.Spans[i].StartMS-translatePadMS), EndMS: job.Spans[i].EndMS + translatePadMS}
			if err := audio.writeWAV(base+".wav", span); err != nil {
				return nil, err
			}
			args = append(args, "-f", base+".wav", "-of", base)
		}
		// #nosec G204 -- the pinned whisper-cli receives generated local paths.
		cmd := exec.CommandContext(ctx, cli, args...)
		cmd.Dir = filepath.Dir(cli)
		if raw, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("whisper-cli: %w: %s", err, lastChars(string(raw), 400))
		}
		for i := from; i < to; i++ {
			if out[i], err = readTranslation(filepath.Join(dir, fmt.Sprintf("s%05d.json", i))); err != nil {
				return nil, err
			}
		}
		if job.Progress != nil {
			job.Progress(float64(to) / float64(len(job.Spans)))
		}
	}
	return out, nil
}

func readTranslation(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- an output this run wrote.
	if err != nil {
		return "", err
	}
	defer f.Close()
	lines, err := parseWhisperJSON(f)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = l.Text
	}
	return strings.Join(parts, " "), nil
}

// parseWhisperJSON reads whisper-cli's -oj output.
func parseWhisperJSON(r io.Reader) ([]Line, error) {
	var doc struct {
		Transcription []struct {
			Offsets struct {
				From int64 `json:"from"`
				To   int64 `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.NewDecoder(io.LimitReader(r, 64<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("read whisper output: %w", err)
	}
	out := []Line{}
	for _, s := range doc.Transcription {
		text := strings.Join(strings.Fields(s.Text), " ")
		if text == "" || s.Offsets.To < s.Offsets.From {
			continue
		}
		if runes := []rune(text); len(runes) > library.MaxVoiceLineRunes {
			text = string(runes[:library.MaxVoiceLineRunes])
		}
		out = append(out, Line{StartMS: s.Offsets.From, EndMS: s.Offsets.To, Text: text})
	}
	return out, nil
}

// keepSpoken drops lines under a third covered by detected speech: a model fills silence with stock phrases.
func keepSpoken(lines []Line, speech []library.VoiceSpan) []Line {
	out := []Line{}
	for _, l := range lines {
		var covered int64
		for _, s := range speech {
			covered += max(0, min(l.EndMS, s.EndMS)-max(l.StartMS, s.StartMS))
		}
		if length := l.EndMS - l.StartMS; length <= 0 || covered*3 >= length {
			out = append(out, l)
		}
	}
	return out
}
