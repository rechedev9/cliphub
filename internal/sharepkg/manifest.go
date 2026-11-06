// Package sharepkg reads and writes a .cliphub share package: a zip with a
// scouting report plus, for every match it names, the match
// record, the tactical document and the position blob, so another Studio can
// replay those rounds without the .dem file.
//
// The layout is fixed and checked strictly on read:
//
//	manifest.json                    Manifest
//	matches/<match id>/tactical.json tacticalplan.Document of the match
//	matches/<match id>/positions.zvpos its position blob
package sharepkg

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const (
	Format    = "cliphub-share"
	Version   = 1
	Extension = ".cliphub"
	MediaType = "application/zip"

	KindReport = "report"

	MaxTitleRunes = 120
	MaxNotesRunes = 4000
	// MaxMatches is the most matches one package carries: a report's cap.
	MaxMatches     = library.MaxReportMatches
	MaxAnnotations = 5000
)

// ErrInvalid wraps every reason a package is refused, so a caller can tell a
// bad file from a failure of its own.
var ErrInvalid = errors.New("invalid share package")

// Manifest is manifest.json. Report is set, as Kind says; every match id it
// names is one of Matches.
type Manifest struct {
	Format      string               `json:"format"`
	Version     int                  `json:"version"`
	CreatedAt   time.Time            `json:"created_at"`
	Kind        string               `json:"kind"`
	Title       string               `json:"title"`
	Notes       string               `json:"notes"`
	Report      *library.Report      `json:"report,omitempty"`
	Matches     []Match              `json:"matches"`
	Annotations []library.Annotation `json:"annotations"`
}

// Match is one match the package carries: its library record and what the
// receiving library needs to register it without the demo file.
type Match struct {
	Match library.Match `json:"match"`
	Demo  Demo          `json:"demo"`
}

// Demo identifies the demo a match was read from. SHA256 is the dedupe key.
type Demo struct {
	SHA256       string `json:"sha256"`
	FileName     string `json:"file_name"`
	CS2Build     int    `json:"cs2_build,omitempty"`
	PatchVersion int    `json:"patch_version,omitempty"`
}

var (
	sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
	mapName   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Validate checks every field a receiving library relies on. It never trusts
// counts or names it has not bounded.
func (m Manifest) Validate() error {
	if m.Format != Format {
		return invalidf("not a ClipHub share package")
	}
	if m.Version > Version {
		return invalidf("made by a newer Studio (version %d); update Studio to open it", m.Version)
	}
	if m.Version != Version {
		return invalidf("unsupported version %d", m.Version)
	}
	if title := strings.TrimSpace(m.Title); title == "" || len([]rune(title)) > MaxTitleRunes {
		return invalidf("title must have from 1 to %d characters", MaxTitleRunes)
	}
	if len([]rune(m.Notes)) > MaxNotesRunes {
		return invalidf("notes must be at most %d characters", MaxNotesRunes)
	}
	if len(m.Matches) == 0 || len(m.Matches) > MaxMatches {
		return invalidf("a package carries from 1 to %d matches", MaxMatches)
	}
	matches := map[uuid.UUID]library.Match{}
	for i, entry := range m.Matches {
		if err := entry.validate(); err != nil {
			return invalidf("match %d: %v", i, err)
		}
		if _, dup := matches[entry.Match.ID]; dup {
			return invalidf("match %s appears twice", entry.Match.ID)
		}
		matches[entry.Match.ID] = entry.Match
	}
	switch m.Kind {
	case KindReport:
		if m.Report == nil {
			return invalidf("a report package carries one report")
		}
		if err := validateReport(*m.Report, matches); err != nil {
			return err
		}
	default:
		return invalidf("kind must be %q", KindReport)
	}
	if len(m.Annotations) > MaxAnnotations {
		return invalidf("a package carries at most %d notes", MaxAnnotations)
	}
	for i, a := range m.Annotations {
		match, ok := matches[a.MatchID]
		if !ok {
			return invalidf("note %d names a match the package does not carry", i)
		}
		if _, err := library.CleanAnnotation(a); err != nil || a.Round > match.RoundCount {
			return invalidf("note %d is not valid", i)
		}
	}
	return nil
}

func (e Match) validate() error {
	m := e.Match
	switch {
	case m.ID == uuid.Nil:
		return errors.New("needs an id")
	case !sha256Hex.MatchString(e.Demo.SHA256):
		return errors.New("demo sha256 must be 64 lowercase hex digits")
	case !mapName.MatchString(m.Map):
		return errors.New("map name is not valid")
	case m.RoundCount < 1 || m.RoundCount > 999:
		return errors.New("round count is out of range")
	case len(m.Teams) > 2 || len(m.Players) > 64:
		return errors.New("too many teams or players")
	case len([]rune(e.Demo.FileName)) > 255:
		return errors.New("the demo file name is too long")
	}
	for _, t := range m.Teams {
		if t.Key != "ct-start" && t.Key != "t-start" {
			return fmt.Errorf("team key %q is not valid", t.Key)
		}
		if len([]rune(t.TeamName)) > MaxTitleRunes || len([]rune(t.ClanName)) > MaxTitleRunes {
			return errors.New("a team name is too long")
		}
	}
	return nil
}

func validateReport(r library.Report, matches map[uuid.UUID]library.Match) error {
	switch {
	case r.ID == uuid.Nil:
		return invalidf("report needs an id")
	case strings.TrimSpace(r.Title) == "" || len([]rune(r.Title)) > MaxTitleRunes:
		return invalidf("report title must have from 1 to %d characters", MaxTitleRunes)
	case strings.TrimSpace(r.TeamName) == "" || len([]rune(r.TeamName)) > MaxTitleRunes:
		return invalidf("report needs the name of its team")
	case r.Side != "" && r.Side != "CT" && r.Side != "T":
		return invalidf("report side must be CT, T or empty")
	case len([]rune(r.Notes)) > MaxNotesRunes:
		return invalidf("report notes are too long")
	case len(r.MatchIDs) > MaxMatches:
		return invalidf("a report reads at most %d matches", MaxMatches)
	}
	for _, id := range r.MatchIDs {
		if _, ok := matches[id]; !ok {
			return invalidf("the report names a match the package does not carry")
		}
	}
	return nil
}
