// demo-corpus exercises local demo parsing and recording-plan preparation.
// It never starts CS2, HLAE, FFmpeg, Studio, or a production job.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/rechedev9/cliphub/internal/demozstd"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/rules"
)

const maxDemoBytes int64 = 700 << 20

type source struct {
	Path        string `json:"path"`
	Prepared    string `json:"prepared"`
	SHA256      string `json:"sha256"`
	Bytes       int64  `json:"bytes"`
	DuplicateOf string `json:"duplicate_of,omitempty"`
	Error       string `json:"error,omitempty"`
}
type check struct {
	Name         string             `json:"name"`
	Status       string             `json:"status"`
	Segments     int                `json:"segments,omitempty"`
	Error        string             `json:"error,omitempty"`
	Notices      []recapplan.Notice `json:"notices,omitempty"`
	Diff         []string           `json:"diff,omitempty"`
	ScriptSHA256 string             `json:"script_sha256,omitempty"`
}
type targetResult struct {
	Player          parser.PlayerStat `json:"player"`
	Seconds         float64           `json:"seconds"`
	ParseError      string            `json:"parse_error,omitempty"`
	FactsError      string            `json:"facts_error,omitempty"`
	TickRate        int               `json:"tick_rate"`
	FirstPacketTick *int              `json:"first_packet_tick,omitempty"`
	Flags           map[string]int    `json:"flags"`
	Checks          []check           `json:"checks"`
}
type demoResult struct {
	Source  source           `json:"source"`
	Started string           `json:"started"`
	Seconds float64          `json:"seconds"`
	Match   parser.MatchInfo `json:"match"`
	Targets []targetResult   `json:"targets"`
	Error   string           `json:"error,omitempty"`
}

func main() {
	input := flag.String("input", "", "directory of .dem and .dem.zst files (recursive)")
	out := flag.String("out", "", "new private report directory outside input")
	workers := flag.Int("workers", 2, "simultaneous demo processes (1-4)")
	timeout := flag.Duration("timeout", 10*time.Minute, "timeout per demo process")
	workerDemo := flag.String("worker-demo", "", "internal isolated parser process")
	digest := flag.String("sha", "", "internal decoded demo digest")
	flag.Parse()
	if *workerDemo != "" {
		if *out == "" || !recapplan.ValidHash(*digest) {
			fail(fmt.Errorf("invalid worker arguments"))
		}
		r := analyze(source{Path: *workerDemo, Prepared: *workerDemo, SHA256: *digest}, *out)
		if err := writeJSON(filepath.Join(*out, "report.json"), r); err != nil {
			fail(err)
		}
		if r.Error != "" {
			os.Exit(1)
		}
		return
	}
	if *input == "" || *out == "" || *workers < 1 || *workers > 4 || *timeout <= 0 {
		fail(fmt.Errorf("usage: demo-corpus --input <demos-dir> --out <new-report-dir> [--workers 2] [--timeout 10m]"))
	}
	if err := run(*input, *out, *workers, *timeout); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func cloneJSON[T any](value T) (T, error) {
	var out T
	b, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(b, &out)
	}
	return out, err
}
func run(input, out string, workers int, timeout time.Duration) error {
	input, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	resolvedInput, err := filepath.EvalSymlinks(input)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return err
	}
	out = filepath.Join(parent, filepath.Base(out))
	rel, err := filepath.Rel(resolvedInput, out)
	if err != nil {
		return err
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel)) {
		return fmt.Errorf("report directory must be outside input")
	}
	if _, err = os.Stat(out); err == nil {
		return fmt.Errorf("report directory already exists: %s", out)
	} else if !os.IsNotExist(err) {
		return err
	}
	var paths []string
	err = filepath.WalkDir(input, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".dem") || strings.HasSuffix(name, ".dem.zst") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no demos found")
	}
	if err = os.Mkdir(out, 0700); err != nil {
		return err
	}
	var sources []source
	unique := map[string]string{}
	for _, path := range paths {
		s := prepare(path, out)
		if s.Error == "" {
			if first, ok := unique[s.SHA256]; ok {
				s.DuplicateOf = first
			} else {
				unique[s.SHA256] = path
			}
		}
		sources = append(sources, s)
	}
	if err = writeJSON(filepath.Join(out, "inventory.json"), sources); err != nil {
		return err
	}
	fmt.Printf("Inventory: %d sources, %d unique decoded demos\n", len(sources), len(unique))
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	queue := make(chan source)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []demoResult
	for i := 0; i < workers; i++ {
		wg.Go(func() {
			for s := range queue {
				folder := filepath.Join(out, s.SHA256)
				_ = os.Mkdir(folder, 0700)
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				cmd := exec.CommandContext(ctx, exe, "--worker-demo", s.Prepared, "--sha", s.SHA256, "--out", folder)
				cmd.Env = append(os.Environ(), "GOMAXPROCS=2", "GOMEMLIMIT=2GiB")
				log, runErr := cmd.CombinedOutput()
				cancel()
				_ = os.WriteFile(filepath.Join(folder, "process.log"), log, 0600)
				var result demoResult
				b, readErr := os.ReadFile(filepath.Join(folder, "report.json"))
				if readErr == nil {
					readErr = json.Unmarshal(b, &result)
				}
				if readErr != nil {
					result.Error = fmt.Sprintf("missing worker report: %v", readErr)
				}
				if runErr != nil {
					result.Error = strings.TrimSpace(result.Error + "; worker: " + runErr.Error())
				}
				result.Source = s
				_ = writeJSON(filepath.Join(folder, "report.json"), result)
				mu.Lock()
				results = append(results, result)
				fmt.Printf("[%d/%d] %s: players=%d, %.1fs, error=%q\n", len(results), len(unique), filepath.Base(s.Path), len(result.Targets), result.Seconds, result.Error)
				mu.Unlock()
			}
		})
	}
	for _, s := range sources {
		if s.Error == "" && s.DuplicateOf == "" {
			queue <- s
		}
	}
	close(queue)
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Source.Path < results[j].Source.Path })
	return writeJSON(filepath.Join(out, "summary.json"), map[string]any{"schema_version": 1, "completed_at": time.Now().UTC().Format(time.RFC3339), "sources": sources, "demos": results, "scope": "all roster players; kills plus Full Demo facts; recording plans, JSON round trips, fingerprints, HLAE script generation. No native capture, voice extraction or render."})
}
func prepare(path, out string) (s source) {
	s = source{Path: path, Prepared: path}
	f, err := os.Open(path)
	if err != nil {
		s.Error = err.Error()
		return
	}
	defer f.Close()
	var header [4]byte
	if _, err := f.ReadAt(header[:], 0); err != nil && err != io.EOF {
		s.Error = err.Error()
		return
	}
	r, _, err := demozstd.Open(f, filepath.Base(path), maxDemoBytes)
	if err != nil {
		s.Error = err.Error()
		return
	}
	defer r.Close()
	h := sha256.New()
	var w io.Writer = h
	if strings.HasSuffix(strings.ToLower(path), ".zst") || bytes.Equal(header[:], []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		pathHash := sha256.Sum256([]byte(path))
		s.Prepared = filepath.Join(out, "prepared", hex.EncodeToString(pathHash[:])+".dem")
		if err = os.MkdirAll(filepath.Dir(s.Prepared), 0700); err != nil {
			s.Error = err.Error()
			return
		}
		decoded, e := os.OpenFile(s.Prepared, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			s.Error = e.Error()
			return
		}
		defer decoded.Close()
		w = io.MultiWriter(h, decoded)
	}
	s.Bytes, err = io.Copy(w, r)
	if err != nil {
		s.Error = err.Error()
		return
	}
	if s.Bytes > maxDemoBytes {
		s.Error = "decoded demo exceeds Studio's 700 MiB limit"
		return
	}
	s.SHA256 = hex.EncodeToString(h.Sum(nil))
	return
}
func analyze(s source, out string) (result demoResult) {
	start := time.Now()
	result = demoResult{Source: s, Started: start.UTC().Format(time.RFC3339), Targets: []targetResult{}}
	defer func() {
		result.Seconds = time.Since(start).Seconds()
		if p := recover(); p != nil {
			result.Error = fmt.Sprintf("panic: %v", p)
			_ = os.WriteFile(filepath.Join(out, "panic.txt"), debug.Stack(), 0600)
		}
	}()
	f, err := os.Open(s.Prepared)
	if err != nil {
		result.Error = err.Error()
		return
	}
	p := demoinfocs.NewParser(f)
	roster, err := parser.RosterScan(p)
	p.Close()
	f.Close()
	if err != nil {
		result.Error = err.Error()
		return
	}
	result.Match = roster.Match
	if err = writeJSON(filepath.Join(out, "roster.json"), roster); err != nil {
		result.Error = err.Error()
		return
	}
	for _, player := range roster.Players {
		if player.SteamID64 == "" || player.SteamID64 == "0" {
			continue
		}
		tr := analyzeTarget(s, player, filepath.Join(out, player.SteamID64))
		result.Targets = append(result.Targets, tr)
		result.Seconds = time.Since(start).Seconds()
		if err = writeJSON(filepath.Join(out, "report.json"), result); err != nil {
			result.Error = err.Error()
			return
		}
	}
	return
}
func analyzeTarget(s source, player parser.PlayerStat, out string) (result targetResult) {
	start := time.Now()
	result = targetResult{Player: player, Flags: map[string]int{}, Checks: []check{}}
	defer func() {
		result.Seconds = time.Since(start).Seconds()
		if p := recover(); p != nil {
			result.ParseError = fmt.Sprintf("panic: %v", p)
			_ = os.MkdirAll(out, 0700)
			_ = os.WriteFile(filepath.Join(out, "panic.txt"), debug.Stack(), 0600)
		}
	}()
	f, err := os.Open(s.Prepared)
	if err != nil {
		result.ParseError = err.Error()
		return
	}
	defer f.Close()
	p := demoinfocs.NewParser(f)
	defer p.Close()
	dual, err := parser.RunKillsAndRecapWithContext(context.Background(), p, player.SteamID64, rules.Default(), parser.PlanMeta{DemoPath: s.Prepared, SHA256: s.SHA256})
	if err != nil {
		result.ParseError = err.Error()
		return
	}
	if err = writeJSON(filepath.Join(out, "parsed.json"), dual); err != nil {
		result.ParseError = err.Error()
		return
	}
	result.TickRate = dual.Kills.Demo.Tickrate
	result.FirstPacketTick = dual.Kills.Demo.FirstFullPacketTick
	if err = dual.Facts.Validate(); err != nil {
		result.FactsError = err.Error()
	}
	for _, r := range dual.Facts.Rounds {
		result.Flags["rounds"]++
		if len(r.Kills) == 0 {
			result.Flags["zero_kills"]++
		}
		if len(r.Utility) == 0 {
			result.Flags["zero_utility"]++
		}
		if r.Number > 24 {
			result.Flags["round_number_above_24"]++
		}
		if r.FreezeEndTick-r.StartTick < 4*dual.Facts.TickRate {
			result.Flags["freeze_below_4s"]++
		}
		if r.Evidence != "round-events" {
			result.Flags["incomplete_round_events"]++
		}
		if r.DeathTick != nil {
			result.Flags["death"]++
			if *r.DeathTick < r.FreezeEndTick {
				result.Flags["freeze_death"]++
			}
			if *r.DeathTick-r.FreezeEndTick < 2*dual.Facts.TickRate {
				result.Flags["death_before_2s_live"]++
			}
		}
	}
	base, err := cloneJSON(dual.Kills)
	if err != nil {
		result.ParseError = err.Error()
		return
	}
	for _, enc := range []string{"", recording.EncoderNVENC} {
		for _, hud := range []recording.HUDMode{recording.HUDModeClean, recording.HUDModeDeathnotices, recording.HUDModeGameplay} {
			stream := recording.DefaultStreamConfig()
			stream.Encoder = enc
			stream.HUDMode = hud
			name := "kills/" + string(hud) + "/" + encoderName(enc)
			result.Checks = append(result.Checks, checkRecording(base, nil, stream, out, name))
		}
	}
	if result.FactsError != "" {
		return
	}
	for _, allowDefault := range []bool{false, true} {
		name := "full-demo/observed"
		if allowDefault {
			name = "full-demo/capture-default-approved"
		}
		opts := recapplan.DefaultOptions()
		opts.Audio.Voice.Enabled = false
		opts.Audio.Music.Enabled = false
		opts.Sponsor.Enabled = false
		opts.Capture.Crosshair.AllowCaptureDefault = allowDefault
		doc, err := recapplan.Plan(dual.Facts, opts, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
		if err != nil {
			result.Checks = append(result.Checks, check{Name: name, Status: "error", Error: err.Error()})
			continue
		}
		if err = doc.Validate(); err != nil {
			result.Checks = append(result.Checks, check{Name: name, Status: "error", Error: err.Error()})
			continue
		}
		// Studio saves the approved document and reloads it before the worker.
		doc, err = cloneJSON(doc)
		if err != nil {
			result.Checks = append(result.Checks, check{Name: name, Status: "error", Error: err.Error()})
			continue
		}
		if err = writeJSON(filepath.Join(out, strings.ReplaceAll(name, "/", "-")+".json"), doc); err != nil {
			result.Checks = append(result.Checks, check{Name: name, Status: "error", Error: err.Error()})
			continue
		}
		if len(doc.Blockers) > 0 {
			result.Checks = append(result.Checks, check{Name: name, Status: "blocked", Segments: len(doc.Rounds), Notices: doc.Blockers})
			continue
		}
		for _, enc := range []string{"", recording.EncoderNVENC} {
			stream := recording.DefaultStreamConfig()
			stream.Encoder = enc
			stream.HUDMode = recording.HUDModeGameplay
			result.Checks = append(result.Checks, checkRecording(doc.KillPlan(base), &doc, stream, out, name+"/"+encoderName(enc)))
		}
	}
	return
}
func encoderName(enc string) string {
	if enc == "" {
		return "software"
	}
	return enc
}
func checkRecording(kp killplan.Plan, doc *recapplan.Document, stream recording.StreamConfig, out, name string) (c check) {
	c = check{Name: name, Status: "passed", Segments: len(kp.Segments)}
	if len(kp.Segments) == 0 {
		c.Status = "empty"
		return
	}
	reject := func(err error) check { c.Status = "error"; c.Error = err.Error(); return c }
	profile, err := recording.NewPlanFromKillPlan(kp, "profile.dem", "profile", stream)
	if err != nil {
		return reject(err)
	}
	captureOut := filepath.Join(out, "capture-not-executed")
	expected, err := recording.NewPlanFromKillPlan(kp, kp.Demo.Path, captureOut, profile.Stream, doc)
	if err != nil {
		return reject(err)
	}
	childKP, err := cloneJSON(kp)
	if err != nil {
		return reject(err)
	}
	childDoc, err := cloneJSON(doc)
	if err != nil {
		return reject(err)
	}
	actual, err := recording.NewPlanFromKillPlan(childKP, kp.Demo.Path, captureOut, stream, childDoc)
	if err != nil {
		return reject(err)
	}
	actual, err = cloneJSON(actual)
	if err != nil {
		return reject(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		c.Diff = diffValues("plan", reflect.ValueOf(expected), reflect.ValueOf(actual))
		_ = writeJSON(filepath.Join(out, strings.ReplaceAll(name, "/", "-")+"-mismatch.json"), map[string]any{"expected": expected, "actual": actual, "diff": c.Diff})
		return reject(fmt.Errorf("recording plan changed across recorder JSON transport"))
	}
	want, err := recording.CaptureInputFingerprint(expected)
	if err != nil {
		return reject(err)
	}
	got, err := recording.CaptureInputFingerprint(actual)
	if err != nil {
		return reject(err)
	}
	if want != got {
		return reject(fmt.Errorf("capture fingerprint changed across transport"))
	}
	script, err := recording.GenerateHLAEJavaScriptWithAttestation(actual, "offline-corpus-only")
	if err != nil {
		return reject(err)
	}
	digest := sha256.Sum256([]byte(script))
	c.ScriptSHA256 = hex.EncodeToString(digest[:])
	return
}
func diffValues(path string, a, b reflect.Value) []string {
	if reflect.DeepEqual(a.Interface(), b.Interface()) {
		return nil
	}
	var diffs []string
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if a.Field(i).CanInterface() {
				diffs = append(diffs, diffValues(path+"."+a.Type().Field(i).Name, a.Field(i), b.Field(i))...)
			}
		}
	case reflect.Pointer:
		if !a.IsNil() && !b.IsNil() {
			return diffValues(path, a.Elem(), b.Elem())
		}
		return []string{path}
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return []string{fmt.Sprintf("%s (nil/length differs)", path)}
		}
		for i := 0; i < a.Len(); i++ {
			diffs = append(diffs, diffValues(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i))...)
		}
	default:
		return []string{path}
	}
	return diffs
}
