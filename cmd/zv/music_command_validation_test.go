package main

import (
	"strings"
	"testing"
)

func TestValidateMusicAnalyzeAcceptsRankingContract(t *testing.T) {
	command := []string{
		"music", "analyze",
		"--input", "track.mp4",
		"--out", "rhythm.json",
		"--recording-result", "recording-result.json",
		"--tail-trim", "1.5",
		"--rank-moments",
		"--limit", "5",
	}
	if issue := validateSkillCommand(command); issue != "" {
		t.Fatalf("validateSkillCommand issue = %q, want none", issue)
	}
	workflow, ok := findWorkflow("music-analyze")
	if !ok {
		t.Fatal("music-analyze workflow missing")
	}
	if issue := validateWorkflowRunForwardedArgs(workflow, append([]string{"--"}, command[2:]...)); issue != "" {
		t.Fatalf("workflow validation issue = %q, want none", issue)
	}
}

func TestValidateMusicAnalyzeRejectsInvalidRankingContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "limit without ranking",
			args: []string{"--input", "track.mp4", "--out", "rhythm.json", "--limit", "5"},
			want: "--limit requires --rank-moments",
		},
		{
			name: "negative limit",
			args: []string{"--input", "track.mp4", "--out", "rhythm.json", "--rank-moments", "--limit", "-1"},
			want: "--limit must be >= 0",
		},
		{
			name: "invalid ranking boolean",
			args: []string{"--input", "track.mp4", "--out", "rhythm.json", "--rank-moments=maybe"},
			want: `invalid boolean value "maybe"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issue := validateMusicAnalyzeCommand(test.args)
			if !strings.Contains(issue, test.want) {
				t.Fatalf("issue = %q, want it to contain %q", issue, test.want)
			}
		})
	}
}

func TestValidateMusicAnalyzeRejectsCompetingTimingSourcesAcrossDirectAndWorkflowPreflight(t *testing.T) {
	args := []string{
		"--input", "track.mp4",
		"--out", "rhythm.json",
		"--killplan", "killplan.json",
		"--recording-result", "recording-result.json",
	}
	const want = "--killplan and --recording-result are mutually exclusive"
	if issue := validateSkillCommand(append([]string{"music", "analyze"}, args...)); !strings.Contains(issue, want) {
		t.Fatalf("direct issue = %q, want it to contain %q", issue, want)
	}
	workflow, ok := findWorkflow("music-analyze")
	if !ok {
		t.Fatal("music-analyze workflow missing")
	}
	if issue := validateWorkflowRunForwardedArgs(workflow, append([]string{"--"}, args...)); !strings.Contains(issue, want) {
		t.Fatalf("workflow issue = %q, want it to contain %q", issue, want)
	}
}
