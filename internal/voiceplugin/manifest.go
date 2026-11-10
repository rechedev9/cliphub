// Package voiceplugin reads what the ClipHub TeamSpeak plugin writes into Studio's inbox: one
// folder per recording with a manifest and, per person, the speech as raw PCM plus where it goes.
package voiceplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

// SampleRate is the only rate the plugin writes: TeamSpeak's own.
const SampleRate = 48000

// The manifest's states: the plugin rewrites a recording's manifest as complete when it stops.
const (
	StateRecording = "recording"
	StateComplete  = "complete"
)

const (
	maxManifestBytes = 1 << 20
	maxTracks        = 32
)

// folderPattern is the name the plugin gives a recording's folder: its UTC start and 4 random hex digits.
var folderPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{4}$`)

// filePattern keeps a track's file inside its folder.
var filePattern = regexp.MustCompile(`^[0-9]{2}-[^/\\:*?"<>|]{1,60}$`)

// ValidID reports whether id can be a recording folder.
func ValidID(id string) bool { return folderPattern.MatchString(id) }

// Manifest is manifest.json as the plugin writes it.
type Manifest struct {
	Format        int    `json:"format"`
	PluginVersion string `json:"plugin_version"`
	State         string `json:"state"`
	Interrupted   bool   `json:"interrupted"`
	StartedAtMS   int64  `json:"started_at_ms"`
	EndedAtMS     int64  `json:"ended_at_ms"`
	SampleRate    int    `json:"sample_rate"`
	Trigger       string `json:"trigger"`
	Server        string `json:"server"`
	Channel       string `json:"channel"`
	Map           string `json:"map"`
	SelfSteamID64 string `json:"self_steamid64"`
	Session       string `json:"session"`
	ScoreCT       int    `json:"score_ct"`
	ScoreT        int    `json:"score_t"`
	Rounds        []struct {
		Round    int   `json:"round"`
		LiveAtMS int64 `json:"live_at_ms"`
	} `json:"rounds"`
	Tracks []struct {
		File     string `json:"file"`
		Nickname string `json:"nickname"`
		UID      string `json:"uid"`
		Self     bool   `json:"self"`
	} `json:"tracks"`
}

// ReadManifest reads and checks a recording folder's manifest.
func ReadManifest(dir string) (Manifest, error) {
	f, err := os.Open(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, maxManifestBytes+1))
	if err != nil {
		return Manifest{}, err
	}
	if len(body) > maxManifestBytes {
		return Manifest{}, errors.New("voiceplugin: manifest too large")
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, fmt.Errorf("voiceplugin: manifest: %w", err)
	}
	return m, m.check()
}

func (m Manifest) check() error {
	switch {
	case m.Format != 1:
		return fmt.Errorf("voiceplugin: manifest format %d is not supported", m.Format)
	case m.SampleRate != SampleRate:
		return fmt.Errorf("voiceplugin: sample rate %d is not supported", m.SampleRate)
	case m.StartedAtMS <= 0:
		return errors.New("voiceplugin: manifest has no start time")
	case len(m.Tracks) > maxTracks:
		return fmt.Errorf("voiceplugin: %d tracks is more than %d", len(m.Tracks), maxTracks)
	}
	for _, t := range m.Tracks {
		if !filePattern.MatchString(t.File) || strings.Contains(t.File, "..") {
			return fmt.Errorf("voiceplugin: track file %q is not a plugin track name", t.File)
		}
	}
	return nil
}

// Recording is the manifest as Studio stores it; durations come from the tracks once read.
func (m Manifest) Recording(id string) library.VoiceRecording {
	rec := library.VoiceRecording{ID: id, StartedAt: time.UnixMilli(m.StartedAtMS).UTC(), EndedAt: time.UnixMilli(max(m.EndedAtMS, m.StartedAtMS)).UTC(),
		Map: library.CleanName(m.Map), Server: clip(m.Server, 120), Channel: clip(m.Channel, 120), Trigger: m.Trigger,
		SelfSteamID64: m.SelfSteamID64, ScoreCT: m.ScoreCT, ScoreT: m.ScoreT, Interrupted: m.Interrupted,
		Tracks: []library.RecordingTrack{}, Rounds: []library.RecordingRound{}}
	if !steamIDPattern.MatchString(rec.SelfSteamID64) {
		rec.SelfSteamID64 = ""
	}
	if rec.Trigger != "manual" {
		rec.Trigger = "auto"
	}
	for _, r := range m.Rounds {
		if r.Round > 0 && r.LiveAtMS > 0 {
			rec.Rounds = append(rec.Rounds, library.RecordingRound{Round: r.Round, LiveAt: time.UnixMilli(r.LiveAtMS).UTC()})
		}
	}
	for _, t := range m.Tracks {
		nick := clip(t.Nickname, library.MaxVoiceLabelRunes)
		if nick == "" {
			nick = t.File
		}
		rec.Tracks = append(rec.Tracks, library.RecordingTrack{File: t.File, Nickname: nick, UID: clip(t.UID, 64), Self: t.Self})
	}
	return rec
}

var steamIDPattern = regexp.MustCompile(`^7656[0-9]{13}$`)

func clip(s string, n int) string {
	s = library.CleanName(s)
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n]))
	}
	return s
}
