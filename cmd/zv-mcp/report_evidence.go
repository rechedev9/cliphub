package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

var supportCodePattern = regexp.MustCompile(`^CH(?:-[A-F0-9]{4}){5}$`)

// reportEvidence is what CONTRIBUTING.md asks an assistant to cite in an issue.
// Every field is best effort: a missing file leaves it empty.
type reportEvidence struct {
	studioVersion string
	supportCode   string
	logPath       string
}

func (s *studio) reportEvidence() reportEvidence {
	dir := filepath.Dir(s.sessionPath)
	var ev reportEvidence
	var sess agentSession
	if raw, err := os.ReadFile(s.sessionPath); err == nil && json.Unmarshal(raw, &sess) == nil {
		ev.studioVersion = cleanText(sess.StudioVersion, 40)
	}
	var tel struct {
		SupportCode string `json:"supportCode"`
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "telemetry.json")); err == nil && json.Unmarshal(raw, &tel) == nil &&
		supportCodePattern.MatchString(tel.SupportCode) {
		ev.supportCode = tel.SupportCode
	}
	if logPath := filepath.Join(dir, "studio.log"); fileExists(logPath) {
		ev.logPath = logPath
	}
	return ev
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
