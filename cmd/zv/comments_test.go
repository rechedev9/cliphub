package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const commentBaselineFile = "cmd/zv/comment-baseline.json"

var shrinkCommentBaseline = flag.Bool("shrink-comment-baseline", false, "lower "+commentBaselineFile+" to the current comment counts")

func goCommentLines(src []byte) (int, []int) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	total := 0
	var starts []int
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return total, starts
		}
		if tok != token.COMMENT || strings.HasPrefix(lit, "//go:") || strings.HasPrefix(lit, "//line ") {
			continue
		}
		total += strings.Count(lit, "\n") + 1
		starts = append(starts, file.Line(pos))
	}
}

func readCommentBaseline(t *testing.T, path string) map[string]int {
	t.Helper()
	frozen := map[string]int{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return frozen
	}
	if err != nil {
		t.Fatalf("read %s: %v", commentBaselineFile, err)
	}
	if err := json.Unmarshal(data, &frozen); err != nil {
		t.Fatalf("parse %s: %v", commentBaselineFile, err)
	}
	return frozen
}

func writeCommentBaseline(t *testing.T, path string, frozen map[string]int) {
	t.Helper()
	if len(frozen) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("remove %s: %v", commentBaselineFile, err)
		}
		return
	}
	data, err := json.MarshalIndent(frozen, "", "  ")
	if err != nil {
		t.Fatalf("encode %s: %v", commentBaselineFile, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", commentBaselineFile, err)
	}
}

func currentGoCommentLines(t *testing.T, root string) (map[string]int, map[string][]int) {
	t.Helper()
	counts := map[string]int{}
	starts := map[string][]int{}
	for _, rel := range gitGoFiles(t, goFileListing{untracked: true}) {
		if strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if lines, at := goCommentLines(data); lines > 0 {
			counts[rel] = lines
			starts[rel] = at
		}
	}
	return counts, starts
}

func TestNoComments(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, filepath.FromSlash(commentBaselineFile))
	frozen := readCommentBaseline(t, path)
	counts, starts := currentGoCommentLines(t, root)
	if *shrinkCommentBaseline {
		next := map[string]int{}
		for rel, limit := range frozen {
			if lines := min(limit, counts[rel]); lines > 0 {
				next[rel] = lines
			}
		}
		writeCommentBaseline(t, path, next)
		frozen = next
	}
	var problems []string
	for rel, lines := range counts {
		limit, listed := frozen[rel]
		switch {
		case !listed:
			problems = append(problems, fmt.Sprintf("%s: comments at lines %v; code has no comments, delete them", rel, starts[rel]))
		case lines > limit:
			problems = append(problems, fmt.Sprintf("%s: %d comment lines, frozen at %d; delete comments, never raise the baseline", rel, lines, limit))
		case lines < limit:
			problems = append(problems, fmt.Sprintf("%s: down to %d comment lines from %d; run go test ./cmd/zv -run TestNoComments -shrink-comment-baseline", rel, lines, limit))
		}
	}
	for rel := range frozen {
		if counts[rel] == 0 {
			problems = append(problems, rel+": no comments left; run go test ./cmd/zv -run TestNoComments -shrink-comment-baseline")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("Go code has no comments (%s freezes older files):\n%s", commentBaselineFile, strings.Join(problems, "\n"))
	}
}
