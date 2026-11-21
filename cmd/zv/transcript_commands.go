package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/transcriptmeasure"
)

type measuredTranscript struct {
	Source string `json:"source"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	transcriptmeasure.Result
}

type transcriptMeasureResult struct {
	OK          bool                     `json:"ok"`
	Library     string                   `json:"library"`
	Transcripts []measuredTranscript     `json:"transcripts"`
	Total       transcriptmeasure.Result `json:"total"`
}

type editedTranscript struct {
	source string
	id     string
	name   string
	frases []transcriptmeasure.Frase
}

func runTranscript(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, transcriptUsage)
		return exitInvalidArgs
	}
	if isHelp(args[0]) {
		fmt.Fprint(stdout, transcriptUsage)
		return exitSuccess
	}
	if args[0] != "measure" {
		fmt.Fprintf(stderr, "unknown transcript command %q\n%s", args[0], transcriptUsage)
		return exitInvalidArgs
	}
	return runTranscriptMeasure(args[1:], stdout, stderr)
}

func runTranscriptMeasure(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, transcriptMeasureUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("transcript measure", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	libraryPath := fs.String("library", "", "path to a Studio library (jobs.db)")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(args, stdout, stderr, err, transcriptMeasureUsage, exitInvalidArgs)
	}
	if fs.NArg() != 0 {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), transcriptMeasureUsage, exitInvalidArgs)
	}
	if strings.TrimSpace(*libraryPath) == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("--library is required"), transcriptMeasureUsage, exitInvalidArgs)
	}
	if *format != "text" && *format != "json" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), transcriptMeasureUsage, exitInvalidArgs)
	}
	absLibrary, _ := filepath.Abs(*libraryPath)
	result, err := measureLibraryTranscripts(context.Background(), absLibrary)
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, "", exitUnexpected)
	}
	if *format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write transcript measure result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	writeTranscriptMeasureText(stdout, result)
	return exitSuccess
}

func measureLibraryTranscripts(ctx context.Context, libraryPath string) (transcriptMeasureResult, error) {
	result := transcriptMeasureResult{OK: true, Library: libraryPath, Transcripts: []measuredTranscript{}}
	repos, err := store.OpenSQLiteLibraryReadOnly(libraryPath)
	if err != nil {
		return result, err
	}
	defer repos.Close()
	edited, err := editedDemoTranscripts(ctx, repos.Library)
	if err != nil {
		return result, err
	}
	var all []transcriptmeasure.Frase
	for _, transcript := range edited {
		result.Transcripts = append(result.Transcripts, measuredTranscript{
			Source: transcript.source, ID: transcript.id, Name: transcript.name,
			Result: transcriptmeasure.Measure(transcript.frases),
		})
		all = append(all, transcript.frases...)
	}
	result.Total = transcriptmeasure.Measure(all)
	return result, nil
}

func editedDemoTranscripts(ctx context.Context, repo store.LibraryRepository) ([]editedTranscript, error) {
	lines, err := repo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{All: true})
	if err != nil {
		return nil, err
	}
	var out []editedTranscript
	for start := 0; start < len(lines); {
		end, edited := start, false
		frases := []transcriptmeasure.Frase{}
		for ; end < len(lines) && lines[end].MatchID == lines[start].MatchID; end++ {
			line := lines[end]
			heard := line.Text
			if line.Corrected {
				heard = line.OriginalText
			}
			edited = edited || line.Corrected || line.Discarded
			frases = append(frases, transcriptmeasure.Frase{
				Heard: heard, Text: line.Text, Discarded: line.Discarded, StartMS: line.StartMS, EndMS: line.EndMS,
			})
		}
		matchID := lines[start].MatchID
		start = end
		if !edited {
			continue
		}
		name, err := demoTranscriptName(ctx, repo, matchID)
		if err != nil {
			return nil, err
		}
		out = append(out, editedTranscript{source: "demo", id: matchID.String(), name: name, frases: frases})
	}
	return out, nil
}

func demoTranscriptName(ctx context.Context, repo store.LibraryRepository, matchID uuid.UUID) (string, error) {
	m, err := repo.GetMatch(ctx, matchID)
	if err != nil {
		return "", fmt.Errorf("read match %s: %w", matchID, err)
	}
	if title := strings.TrimSpace(m.DemoTitle); title != "" {
		return title, nil
	}
	if len(m.Teams) != 2 {
		return m.Map, nil
	}
	return fmt.Sprintf("%s %s %d-%d %s", m.Map, teamLabel(m.Teams[0]), m.Teams[0].Score, m.Teams[1].Score, teamLabel(m.Teams[1])), nil
}

func teamLabel(team library.MatchTeam) string {
	if team.TeamName != "" {
		return team.TeamName
	}
	if team.ClanName != "" {
		return team.ClanName
	}
	return team.Key
}

func writeTranscriptMeasureText(w io.Writer, result transcriptMeasureResult) {
	if len(result.Transcripts) == 0 {
		fmt.Fprintf(w, "no corrected or discarded frases in %s\n", result.Library)
		return
	}
	fmt.Fprintln(w, "source\tid\tname\twords\tcorrected_words\tcorrected_share\tfrases\tdiscarded\tspeech_min\tdiscarded_per_min")
	for _, transcript := range result.Transcripts {
		writeTranscriptMeasureRow(w, []string{transcript.Source, transcript.ID, transcript.Name}, transcript.Result)
	}
	writeTranscriptMeasureRow(w, []string{"total", "-", fmt.Sprintf("%d transcripts", len(result.Transcripts))}, result.Total)
}

func writeTranscriptMeasureRow(w io.Writer, labels []string, r transcriptmeasure.Result) {
	fmt.Fprintf(w, "%s\t%d\t%d\t%.1f%%\t%d\t%d\t%.2f\t%.2f\n", strings.Join(labels, "\t"),
		r.Words, r.CorrectedWords, r.CorrectedShare*100, r.Frases, r.DiscardedFrases, float64(r.SpeechMS)/60_000, r.DiscardedPerMinute)
}
