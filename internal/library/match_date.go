package library

import (
	"strings"
	"time"
)

// MatchDate is the date a match takes from its demo: the one the importer gave, else the one the
// demo shows, else the import time. A demo date after the import cannot be the match's.
func MatchDate(d Demo, shown *time.Time) time.Time {
	switch {
	case d.PlayedAt != nil:
		return *d.PlayedAt
	case shown != nil && !shown.IsZero() && !shown.After(d.ImportedAt):
		return *shown
	}
	return d.ImportedAt
}

// DatedByImport reports whether a match still carries the import time of its demo, the date it
// takes when nothing better is known. A date set by hand does not land on that millisecond.
func DatedByImport(m Match, d Demo) bool {
	return d.PlayedAt == nil && m.PlayedAt.Equal(d.ImportedAt)
}

// FaceitMatchID returns the FACEIT match id ("1-<uuid>") a FACEIT demo file is named after.
func FaceitMatchID(fileName string) (string, bool) {
	id := faceitFileName.FindString(strings.ToLower(fileName))
	return id, id != ""
}
