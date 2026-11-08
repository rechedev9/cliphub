package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
)

// povSwitchTicks is the gap two clips of one demo keep, so the recorder can change POV between them.
const povSwitchTicks = 32

// throwSource is the analysis a throw was read from.
type throwSource struct {
	file     string
	mapName  string
	tickRate int
	duration int
}

// clipPlan is what the operator PC records next: per kept demo, the throw windows,
// and the combos to join once all their throws are recorded.
type clipPlan struct {
	Demos  []clipDemo  `json:"demos"`
	Combos []clipCombo `json:"combos"`
}

type clipDemo struct {
	Demo          string     `json:"demo"`
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

// clipCombo is a combo clip: its throw clips joined in clock order.
type clipCombo struct {
	Name  string   `json:"name"`
	Parts []string `json:"parts"`
}

type clipPlanInput struct {
	lineups  []prolineups.Lineup
	combos   []prolineups.Combo
	sources  map[string]throwSource
	demo     func(file string) (string, bool)
	recorded func(name string) bool
	// failed names throws the recorder could not film; another example is taken instead.
	failed func(name string) bool
	max    int
	// team, when set, plans only that team's throws and combos.
	team string
}

type clipPlanner struct {
	in      clipPlanInput
	byDemo  map[string]*clipDemo
	planned map[string]bool
}

// keptDemo is the kept demo a throw was read from, if there is one and the throw can be filmed.
func (p *clipPlanner) keptDemo(e prolineups.Example) (string, throwSource, bool) {
	name := prolineups.ClipName(e)
	if p.in.failed != nil && p.in.failed(name) {
		return "", throwSource{}, false
	}
	src, ok := p.in.sources[name]
	if !ok {
		return "", throwSource{}, false
	}
	path, kept := p.in.demo(src.file)
	return path, src, kept
}

// add plans one throw clip unless it is recorded or already planned.
func (p *clipPlanner) add(e prolineups.Example, path string, src throwSource) {
	name := prolineups.ClipName(e)
	if p.planned[name] || p.in.recorded(name) || len(p.planned) == p.in.max {
		return
	}
	p.planned[name] = true
	d := p.byDemo[path]
	if d == nil {
		d = &clipDemo{Demo: path, Map: src.mapName, TickRate: src.tickRate, DurationTicks: src.duration}
		p.byDemo[path] = d
	}
	start, end := prolineups.ClipWindow(e.Tick, src.tickRate, e.AirTime)
	d.Clips = append(d.Clips, clipShot{Name: name, SteamID64: e.SteamID64, Round: e.Round, ThrowTick: e.Tick, StartTick: max(start, 1), EndTick: min(end, src.duration)})
}

func (p *clipPlanner) ofTeam(e prolineups.Example) bool {
	return p.in.team == "" || strings.EqualFold(e.Team, p.in.team)
}

// comboThrows are a combo example's throws when every one comes from one kept demo.
func (p *clipPlanner) comboThrows(e prolineups.Example) ([]prolineups.Example, string, throwSource, bool) {
	if prolineups.ComboClipName(e) == "" {
		return nil, "", throwSource{}, false
	}
	parts := []prolineups.Example{}
	var path string
	var src throwSource
	for _, t := range e.Throws {
		part := prolineups.ThrowExample(e, t)
		pp, ps, ok := p.keptDemo(part)
		if !ok || (path != "" && pp != path) {
			return nil, "", throwSource{}, false
		}
		path, src = pp, ps
		parts = append(parts, part)
	}
	return parts, path, src, true
}

// planClips picks, for each lineup and combo without a clip, its newest throws whose demo is kept.
func planClips(in clipPlanInput) clipPlan {
	p := &clipPlanner{in: in, byDemo: map[string]*clipDemo{}, planned: map[string]bool{}}
	for _, l := range in.lineups {
		if slices.ContainsFunc(l.Examples, func(e prolineups.Example) bool { return p.ofTeam(e) && in.recorded(prolineups.ClipName(e)) }) {
			continue
		}
		for _, e := range l.Examples {
			if !p.ofTeam(e) {
				continue
			}
			if path, src, ok := p.keptDemo(e); ok {
				p.add(e, path, src)
				break
			}
		}
	}
	plan := clipPlan{Demos: []clipDemo{}, Combos: []clipCombo{}}
	for _, c := range in.combos {
		if slices.ContainsFunc(c.Examples, func(e prolineups.Example) bool { return p.ofTeam(e) && in.recorded(prolineups.ComboClipName(e)) }) {
			continue
		}
		for _, e := range c.Examples {
			if !p.ofTeam(e) {
				continue
			}
			parts, path, src, ok := p.comboThrows(e)
			if !ok {
				continue
			}
			combo := clipCombo{Name: prolineups.ComboClipName(e)}
			for _, part := range parts {
				p.add(part, path, src)
				combo.Parts = append(combo.Parts, prolineups.ClipName(part))
			}
			plan.Combos = append(plan.Combos, combo)
			break
		}
	}
	for _, d := range p.byDemo {
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

// clipWriteTimeout lets a clip download take minutes on a slow link.
const clipWriteTimeout = 5 * time.Minute

// clipFailed reports whether record-clips marked a throw as one the recorder cannot film.
func clipFailed(dir string) func(name string) bool {
	return func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)) + ".failed")
		return err == nil
	}
}

// handleProClip serves GET /v1/pro-clips/{match}/{file}: one recorded throw clip.
func (a *api) handleProClip(w http.ResponseWriter, r *http.Request) {
	match, file := r.PathValue("match"), r.PathValue("file")
	if a.clipsDir == "" || !prolineups.ValidClipName(match+"/"+file) {
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
	// A clip is megabytes, more than the server's 15 s write timeout carries on a slow link.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(clipWriteTimeout))
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	http.ServeContent(w, r, file, info.ModTime(), f)
}

type clipDirs struct {
	analyses, demos, clips, out, team string
	max                               int
	// picks, when set, names the one team to record on each map; other maps are left out.
	picks map[string]string
}

// planClipsFromDirs builds the lineups from the local analyses and writes what to record next.
func planClipsFromDirs(ctx context.Context, dirs clipDirs) error {
	read, err := readLineupThrows(ctx, dirs.analyses)
	if err != nil {
		return err
	}
	demo := func(file string) (string, bool) {
		path := filepath.Join(dirs.demos, file+".dem")
		info, err := os.Stat(path)
		return path, err == nil && info.Mode().IsRegular()
	}
	plan := clipPlan{Demos: []clipDemo{}, Combos: []clipCombo{}}
	lineups, combos := 0, 0
	for _, m := range slices.Sorted(maps.Keys(read.byMap)) {
		team := dirs.team
		if dirs.picks != nil {
			pick, ok := dirs.picks[m]
			if !ok {
				continue
			}
			team = pick
		}
		idx := prolineups.BuildIndex(m, read.byMap[m], time.Now())
		lineups, combos = lineups+len(idx.Lineups), combos+len(idx.Combos)
		part := planClips(clipPlanInput{lineups: idx.Lineups, combos: idx.Combos, sources: read.sources, recorded: clipRecorded(dirs.clips), failed: clipFailed(dirs.clips), max: dirs.max, team: team, demo: demo})
		plan.Demos, plan.Combos = append(plan.Demos, part.Demos...), append(plan.Combos, part.Combos...)
	}
	n := 0
	for _, d := range plan.Demos {
		n += len(d.Clips)
	}
	log.Printf("community stage=clips class=planned demos=%d clips=%d combos=%d lineups=%d all_combos=%d", len(plan.Demos), n, len(plan.Combos), lineups, combos)
	return writeClipPlan(dirs.out, plan)
}

// readClipPicks reads the harvester's best-maps.json into map key to team name.
func readClipPicks(path string) (map[string]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Map  string `json:"map"`
		Team string `json:"team"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	picks := map[string]string{}
	for _, r := range rows {
		if r.Map != "" && r.Team != "" {
			picks[r.Map] = r.Team
		}
	}
	if len(picks) == 0 {
		return nil, fmt.Errorf("%s names no map and team", path)
	}
	return picks, nil
}
