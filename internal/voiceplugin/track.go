package voiceplugin

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// run is one stretch of speech: Count samples stored next in the .pcm, placed at Start on the recording's timeline.
type run struct {
	Start, Count int64
}

// maxTrackSamples bounds a rebuilt track to twelve hours.
const maxTrackSamples = 12 * 3600 * SampleRate

// Track is one person's audio rebuilt with silence between the runs, ready to encode.
type Track struct {
	pcm  *os.File
	runs []run
	// Samples is the rebuilt length, from the recording's start to the end of the last run.
	Samples int64
}

// OpenTrack reads a track's index and opens its speech. Speech past the index, left by a plugin that
// stopped before writing its last line, continues right after the last run.
func OpenTrack(dir, file string) (*Track, error) {
	if !filePattern.MatchString(file) {
		return nil, fmt.Errorf("voiceplugin: track file %q is not a plugin track name", file)
	}
	runs, err := readIndex(filepath.Join(dir, file+".idx"))
	if err != nil {
		return nil, err
	}
	pcm, err := os.Open(filepath.Join(dir, file+".pcm"))
	if err != nil {
		return nil, err
	}
	info, err := pcm.Stat()
	if err != nil {
		_ = pcm.Close()
		return nil, err
	}
	stored := info.Size() / 2
	t := &Track{pcm: pcm}
	var end, used int64
	for _, r := range runs {
		if used >= stored {
			break
		}
		r.Count = min(r.Count, stored-used)
		r.Start = max(r.Start, end)
		t.runs = append(t.runs, r)
		used += r.Count
		end = r.Start + r.Count
	}
	if used < stored {
		t.runs = append(t.runs, run{Start: end, Count: stored - used})
		end += stored - used
	}
	if end > maxTrackSamples {
		_ = pcm.Close()
		return nil, errors.New("voiceplugin: track is longer than twelve hours")
	}
	t.Samples = end
	return t, nil
}

func readIndex(path string) ([]run, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var runs []run
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 2 {
			continue
		}
		start, err1 := strconv.ParseInt(fields[0], 10, 64)
		count, err2 := strconv.ParseInt(fields[1], 10, 64)
		if err1 != nil || err2 != nil || start < 0 || count <= 0 {
			continue
		}
		runs = append(runs, run{Start: start, Count: count})
	}
	return runs, scan.Err()
}

// DurationMS is the rebuilt length in milliseconds.
func (t *Track) DurationMS() int64 { return t.Samples * 1000 / SampleRate }

// Close releases the speech file.
func (t *Track) Close() error { return t.pcm.Close() }

// PCM streams the rebuilt track as signed 16-bit little-endian samples.
func (t *Track) PCM() io.Reader {
	readers := []io.Reader{}
	var at, offset int64
	for _, r := range t.runs {
		if gap := r.Start - at; gap > 0 {
			readers = append(readers, io.LimitReader(zeros{}, gap*2))
		}
		readers = append(readers, io.NewSectionReader(t.pcm, offset*2, r.Count*2))
		offset += r.Count
		at = r.Start + r.Count
	}
	return io.MultiReader(readers...)
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
