package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rechedev9/tickcut/internal/storage"
)

func parseFormatArgs(args []string) (string, []string, error) {
	format := "text"
	formatSet := false
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--format" {
			if formatSet {
				return "", nil, fmt.Errorf("duplicate flag --format")
			}
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--format requires a value")
			}
			format = args[i+1]
			formatSet = true
			i++
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--format="); ok {
			if formatSet {
				return "", nil, fmt.Errorf("duplicate flag --format")
			}
			format = value
			formatSet = true
			continue
		}
		rest = append(rest, arg)
	}
	if format != "text" && format != "json" {
		return "", nil, fmt.Errorf("unsupported format %q", format)
	}
	return format, rest, nil
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func isSingleHelp(args []string) bool {
	return len(args) == 1 && isHelp(args[0])
}

// writeJSONArtifact writes an indented JSON document through the local storage
// boundary, so a reader never observes a half-written artifact.
func writeJSONArtifact(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeArtifact(path, body)
}

// writeArtifact writes raw bytes through the same atomic boundary.
func writeArtifact(path string, body []byte) error {
	store, err := storage.NewLocal(filepath.Dir(path))
	if err != nil {
		return err
	}
	return store.Put(filepath.Base(path), bytes.NewReader(body))
}

// writeCommandError reports a failure the way every artifact-producing zv
// subcommand does: as a JSON envelope when the caller asked for JSON, and as a
// stderr message plus usage otherwise.
func writeCommandError(args []string, stdout, stderr io.Writer, err error, commandUsage string, code int) int {
	if shortJSONRequested(args) {
		if writeErr := writeJSON(stdout, map[string]any{"ok": false, "executed": false, "error": err.Error()}); writeErr != nil {
			fmt.Fprintf(stderr, "error: write json error: %v\n", writeErr)
			return exitUnexpected
		}
		return code
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	if commandUsage != "" {
		fmt.Fprint(stderr, commandUsage)
	}
	return code
}
