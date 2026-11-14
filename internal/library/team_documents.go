package library

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	// MaxTeamDocumentBytes caps one document: an HTML report with its styles, scripts and inline images.
	MaxTeamDocumentBytes = 5 << 20
	// MaxTeamDocumentTitleRunes caps a document's title.
	MaxTeamDocumentTitleRunes = 120
	maxTeamDocumentFileRunes  = 200
	// TeamDocumentHTML is the one content type a team document has today.
	TeamDocumentHTML = "text/html"
)

// TeamDocument is a file a coach or an assistant attached to a team, such as an
// anti-strat report. Its content is read on its own, so lists stay small.
type TeamDocument struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	TeamName    string    `json:"team_name"`
	Title       string    `json:"title"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TeamDocumentFilter selects documents for a list read.
type TeamDocumentFilter struct {
	TeamID *uuid.UUID
	Limit  int
	Offset int
}

// NewTeamDocumentInput is what a person or an assistant sends to attach a document.
type NewTeamDocumentInput struct {
	TeamID   uuid.UUID
	Title    string
	FileName string
	Content  []byte
}

// NewTeamDocument validates an upload and returns the record to store. The
// title defaults to the file name without its extension.
func NewTeamDocument(in NewTeamDocumentInput) (TeamDocument, error) {
	if len(in.Content) == 0 {
		return TeamDocument{}, errors.New("the document is empty")
	}
	if len(in.Content) > MaxTeamDocumentBytes {
		return TeamDocument{}, fmt.Errorf("the document is larger than %d MiB", MaxTeamDocumentBytes>>20)
	}
	if !utf8.Valid(in.Content) {
		return TeamDocument{}, errors.New("the document is not UTF-8 text")
	}
	if !LooksLikeHTML(in.Content) {
		return TeamDocument{}, errors.New("the document is not HTML")
	}
	fileName := cleanDocumentText(filepath.Base(strings.ReplaceAll(in.FileName, `\`, "/")))
	if fileName == "." || fileName == "/" {
		fileName = ""
	}
	if utf8.RuneCountInString(fileName) > maxTeamDocumentFileRunes {
		return TeamDocument{}, fmt.Errorf("the file name is longer than %d characters", maxTeamDocumentFileRunes)
	}
	title := cleanDocumentText(in.Title)
	if title == "" {
		title = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	if title == "" {
		return TeamDocument{}, errors.New("the document needs a title")
	}
	if utf8.RuneCountInString(title) > MaxTeamDocumentTitleRunes {
		return TeamDocument{}, fmt.Errorf("the title is longer than %d characters", MaxTeamDocumentTitleRunes)
	}
	sum := sha256.Sum256(in.Content)
	return TeamDocument{
		TeamID:      in.TeamID,
		Title:       title,
		FileName:    fileName,
		ContentType: TeamDocumentHTML,
		SizeBytes:   int64(len(in.Content)),
		SHA256:      hex.EncodeToString(sum[:]),
	}, nil
}

// LooksLikeHTML reports whether the start of a file is markup: a doctype, a
// comment or a tag after optional whitespace and a byte order mark.
func LooksLikeHTML(content []byte) bool {
	head := bytes.TrimLeft(bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF")), " \t\r\n\f")
	return len(head) > 1 && head[0] == '<' && (head[1] == '!' || head[1] == '/' || isASCIILetter(head[1]))
}

// HasDoctype reports whether an HTML document declares its doctype, so a viewer
// can add one to a fragment instead of rendering it in quirks mode.
func HasDoctype(content []byte) bool {
	head := bytes.TrimLeft(bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF")), " \t\r\n\f")
	for bytes.HasPrefix(head, []byte("<!--")) {
		end := bytes.Index(head, []byte("-->"))
		if end < 0 {
			return false
		}
		head = bytes.TrimLeft(head[end+3:], " \t\r\n\f")
	}
	return len(head) >= 9 && strings.EqualFold(string(head[:9]), "<!doctype")
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// cleanDocumentText trims a title or file name and drops control characters.
func cleanDocumentText(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s))
}
