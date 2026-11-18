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
	"unicode"

	"github.com/rechedev9/cliphub/internal/library"
)

// Speech regions are cut with a little air around the talk and capped, so a long call stays one file.
const (
	regionPadMS = 200
	regionGapMS = 600
	regionMaxMS = 20_000
	// A pause this long between words, or a phrase this long, starts a new phrase.
	phraseGapSeconds = 0.6
	phraseMaxSeconds = 5.0
	// regionBatch is how many regions one run reads, so progress moves on a long demo.
	regionBatch = 32
)

// Transcriber runs Parakeet on NeMo-Speech.cpp over a recording's speech; the language is detected.
type Transcriber struct{ *tool }

func NewTranscriber(opts TranscriberOptions) *Transcriber {
	return &Transcriber{newTool(opts, Pinned, "La transcripción")}
}

// TranscribeJob is one track to transcribe; nil Speech means the transcriber finds it.
type TranscribeJob struct {
	Audio    string
	Speech   []library.VoiceSpan
	Progress func(share float64)
}

// Line is one transcribed phrase in audio time.
type Line struct {
	StartMS int64
	EndMS   int64
	Text    string
}

// Transcribe returns the phrases of one track. Only stretches with detected speech are read,
// and a phrase mostly over silence is dropped: no text is invented over a quiet track.
func (t *Transcriber) Transcribe(ctx context.Context, job TranscribeJob) ([]Line, error) {
	cli, model, err := t.ready()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "cliphub-voice-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	audio, err := decodeTrack(ctx, t.opts.Media, job.Audio, dir)
	if err != nil {
		return nil, err
	}
	defer audio.Close()
	speech := job.Speech
	if speech == nil {
		if speech, _, err = DetectSpeech(io.NewSectionReader(audio, 0, audio.size)); err != nil {
			return nil, err
		}
	}
	regions := speechRegions(speech, audio.durationMS())
	lines := []Line{}
	for from := 0; from < len(regions); from += regionBatch {
		batch := regions[from:min(len(regions), from+regionBatch)]
		got, err := t.runBatch(ctx, nemoBatch{cli: cli, model: model, dir: filepath.Join(dir, fmt.Sprintf("b%04d", from)), audio: audio, regions: batch})
		if err != nil {
			return nil, err
		}
		lines = append(lines, got...)
		if job.Progress != nil {
			job.Progress(float64(from+len(batch)) / float64(len(regions)))
		}
	}
	return keepSpoken(lines, speech), nil
}

type nemoBatch struct {
	cli, model, dir string
	audio           *pcmFile
	regions         []library.VoiceSpan
}

func (t *Transcriber) runBatch(ctx context.Context, b nemoBatch) ([]Line, error) {
	in, out := filepath.Join(b.dir, "in"), filepath.Join(b.dir, "out")
	for _, d := range []string{in, out} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return nil, err
		}
	}
	byName := map[string]library.VoiceSpan{}
	for i, r := range b.regions {
		name := fmt.Sprintf("r%04d", i)
		byName[name] = r
		if err := b.audio.writeWAV(filepath.Join(in, name+".wav"), r); err != nil {
			return nil, err
		}
	}
	// #nosec G204 -- the pinned nemo-speech receives generated local paths.
	cmd := exec.CommandContext(ctx, b.cli, "transcribe", in, "--model", b.model, "--device", "cpu", "--asr.backend.threads", threads(),
		"--concurrency", "1", "--format", "json", "--output-dir", out, "--force")
	cmd.Dir = filepath.Dir(b.cli)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "NEMO_SPEECH_MODEL_DIR="+filepath.Dir(b.model))
	if raw, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("nemo-speech: %w: %s", err, lastChars(string(raw), 400))
	}
	outputs, err := filepath.Glob(filepath.Join(out, "*.json"))
	if err != nil {
		return nil, err
	}
	var lines []Line
	for _, path := range outputs {
		doc, err := readNemoJSON(path)
		if err != nil {
			return nil, err
		}
		if region, ok := byName[strings.TrimSuffix(filepath.Base(doc.File), filepath.Ext(doc.File))]; ok {
			lines = append(lines, nemoPhrases(doc, region)...)
		}
	}
	return lines, nil
}

type nemoOutput struct {
	File  string `json:"file"`
	Text  string `json:"text"`
	Words []struct {
		Word  string  `json:"word"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"words"`
}

func readNemoJSON(path string) (nemoOutput, error) {
	var doc nemoOutput
	f, err := os.Open(path) // #nosec G304 -- an output this run wrote.
	if err != nil {
		return doc, err
	}
	defer f.Close()
	if err := json.NewDecoder(io.LimitReader(f, 16<<20)).Decode(&doc); err != nil {
		return doc, fmt.Errorf("read nemo-speech output: %w", err)
	}
	return doc, nil
}

// nemoPhrases splits a region's words into phrases at pauses, in track time.
func nemoPhrases(doc nemoOutput, region library.VoiceSpan) []Line {
	if len(doc.Words) == 0 {
		return spoken([]Line{{StartMS: region.StartMS, EndMS: region.EndMS, Text: doc.Text}})
	}
	at := func(seconds float64) int64 { return region.StartMS + int64(seconds*1000+0.5) }
	var out []Line
	var words []string
	var start, end float64
	flush := func() {
		if len(words) > 0 {
			out = append(out, Line{StartMS: at(start), EndMS: at(end), Text: strings.Join(words, " ")})
		}
		words = nil
	}
	for _, w := range doc.Words {
		if len(words) > 0 && (w.Start-end >= phraseGapSeconds || w.End-start > phraseMaxSeconds) {
			flush()
		}
		if len(words) == 0 {
			start = w.Start
		}
		words, end = append(words, w.Word), max(end, w.End)
	}
	flush()
	return spoken(out)
}

// spoken tidies phrases and drops those without a letter or digit.
func spoken(lines []Line) []Line {
	out := []Line{}
	for _, l := range lines {
		l.Text = strings.Join(strings.Fields(l.Text), " ")
		if !strings.ContainsFunc(l.Text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) || l.EndMS < l.StartMS {
			continue
		}
		if runes := []rune(l.Text); len(runes) > library.MaxVoiceLineRunes {
			l.Text = string(runes[:library.MaxVoiceLineRunes])
		}
		out = append(out, l)
	}
	return out
}

// speechRegions pads each stretch of talk, joins those a short pause apart and splits long ones.
func speechRegions(speech []library.VoiceSpan, durationMS int64) []library.VoiceSpan {
	var out []library.VoiceSpan
	for _, s := range speech {
		s.StartMS, s.EndMS = max(0, s.StartMS-regionPadMS), min(durationMS, s.EndMS+regionPadMS)
		if n := len(out); n > 0 {
			last := &out[n-1]
			if s.StartMS-last.EndMS < regionGapMS && s.EndMS-last.StartMS <= regionMaxMS {
				last.EndMS = max(last.EndMS, s.EndMS)
				continue
			}
			s.StartMS = max(s.StartMS, last.EndMS)
		}
		for s.EndMS-s.StartMS > regionMaxMS {
			out = append(out, library.VoiceSpan{StartMS: s.StartMS, EndMS: s.StartMS + regionMaxMS})
			s.StartMS += regionMaxMS
		}
		if s.EndMS > s.StartMS {
			out = append(out, s)
		}
	}
	return out
}
