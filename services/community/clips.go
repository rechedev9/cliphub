package main

import (
	"cmp"
	"context"
	"encoding/json"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
)

// povSwitchTicks is the gap two clips of one demo keep, so the recorder can change POV between them.
const povSwitchTicks = 32

var (
	clipFolder = regexp.MustCompile(`^\d{1,12}$`)
	clipFile   = regexp.MustCompile(`^[a-z0-9-]{1,220}-r\d{1,3}-t\d{1,9}-\d{17}\.mp4$`)
)

// throwSource is the analysis a throw was read from.
type throwSource struct {
	file     string
	mapName  string
	tickRate int
	duration int
}

// clipPlan is what the operator PC records next: per kept demo, the throw windows.
type clipPlan struct {
	Demos []clipDemo `json:"demos"`
}

type clipDemo struct {
	Demo     string     `json:"demo"`
	Map           string     `json:"map"`
	TickRate      int        `json:"tick_rate"`
	DurationTicks int        `json:"duration_ticks"`
	Clips         []clipShot `json:"clips"`
}

type clipShot struct {
	Name      string `json:"name"`
	SteamID64 string `json:"steamid64"`
	Round     int    `json:"round"`
	ThrowTick int    `json:"throw_tick"`
	StartTick int    `json:"start_tick"`
	EndTick   int    `json:"end_tick"`
}

type clipPlanInput struct {
	lineups  []prolineups.Lineup
	sources  map[string]throwSource
	demo     func(file string) (string, bool)
	recorded func(name string) bool
	max      int
}

// planClips picks, for each lineup without a clip, its newest throw whose demo is kept.
func planClips(in clipPlanInput) clipPlan {
	byDemo := map[string]*clipDemo{}
	planned := 0
	for _, l := range in.lineups {
		if planned == in.max || slices.ContainsFunc(l.Examples, func(e prolineups.Example) bool { return in.recorded(prolineups.ClipName(e)) }) {
			continue
		}
		for _, e := range l.Examples {
			name := prolineups.ClipName(e)
			src, ok := in.sources[name]
			if name == "" || !ok {
				continue
			}
			path, kept := in.demo(src.file)
			if !kept {
				continue
			}
			d := byDemo[path]
			if d == nil {
				d = &clipDemo{Demo: path, Map: src.mapName, TickRate: src.tickRate, DurationTicks: src.duration}
				byDemo[path] = d
			}
			start, end := prolineups.ClipWindow(e.Tick, src.tickRate, e.AirTime)
			d.Clips = append(d.Clips, clipShot{Name: name, SteamID64: e.SteamID64, Round: e.Round, ThrowTick: e.Tick, StartTick: max(start, 1), EndTick: min(end, src.duration)})
			planned++
			break
		}
	}
	plan := clipPlan{Demos: []clipDemo{}}
	for _, d := range byDemo {
		slices.SortFunc(d.Clips, func(a, b clipShot) int { return cmp.Compare(a.StartTick, b.StartTick) })
		kept := d.Clips[:0]
		for _, c := range d.Clips {
			if len(kept) > 0 && c.StartTick < kept[len(kept)-1].EndTick+povSwitchTicks {
				continue
			}
			kept = append(kept, c)
		}
		d.Clips = kept
		plan.Demos = append(plan.Demos, *d)
	}
	slices.SortFunc(plan.Demos, func(a, b clipDemo) int { return cmp.Compare(a.Demo, b.Demo) })
	return plan
}

func writeClipPlan(path string, plan clipPlan) error {
	body, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, body)
}

func writeFile(path string, body []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// clipRecorded reports whether a clip file is stored under dir.
func clipRecorded(dir string) func(name string) bool {
	return func(name string) bool {
		if dir == "" || name == "" {
			return false
		}
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		return err == nil && info.Mode().IsRegular() && info.Size() > 0
	}
}

// handleProClip serves GET /v1/pro-clips/{match}/{file}: one recorded throw clip.
func (a *api) handleProClip(w http.ResponseWriter, r *http.Request) {
	match, file := r.PathValue("match"), r.PathValue("file")
	if a.clipsDir == "" || !clipFolder.MatchString(match) || !clipFile.MatchString(file) {
		writeError(w, http.StatusNotFound, "no such clip")
		return
	}
	path := filepath.Join(a.clipsDir, match, file)
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such clip")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "no such clip")
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	http.ServeContent(w, r, file, info.ModTime(), f)
}

type clipDirs struct {
	analyses, demos, clips, out string
	max                         int
}

// planClipsFromDirs builds the lineups from the local analyses and writes what to record next.
func planClipsFromDirs(ctx context.Context, dirs clipDirs) error {
	read, err := readLineupThrows(ctx, dirs.analyses)
	if err != nil {
		return err
	}
	lineups := []prolineups.Lineup{}
	maps := slices.Sorted(maps.Keys(read.byMap))
	for _, m := range maps {
		lineups = append(lineups, prolineups.BuildIndex(m, read.byMap[m], time.Now()).Lineups...)
	}
	plan := planClips(clipPlanInput{lineups: lineups, sources: read.sources, recorded: clipRecorded(dirs.clips), max: dirs.max,
		demo: func(file string) (string, bool) {
			path := filepath.Join(dirs.demos, file+".dem")
			info, err := os.Stat(path)
			return path, err == nil && info.Mode().IsRegular()
		}})
	n := 0
	for _, d := range plan.Demos {
		n += len(d.Clips)
	}
	log.Printf("community stage=clips class=planned demos=%d clips=%d lineups=%d", len(plan.Demos), n, len(lineups))
	return writeClipPlan(dirs.out, plan)
}
