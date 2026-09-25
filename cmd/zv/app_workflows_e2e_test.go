package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZVBinaryEveryWorkflowRunCommandEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	writeSkillBody(t, tempDir, "alpha", strings.Join([]string{
		"---",
		"name: alpha",
		`description: "Alpha workflow"`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		"```",
		"",
	}, "\n"))
	writeWorkflowDocs(t, tempDir)
	if err := os.MkdirAll(filepath.Join(tempDir, "gallery"), 0o755); err != nil {
		t.Fatalf("mkdir gallery: %v", err)
	}
	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	writeFile(t, galleryPath, "<!doctype html><title>gallery</title>\n")

	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	subcommandLogPath := filepath.Join(tempDir, "subcommands.jsonl")
	openPathLogPath := filepath.Join(tempDir, "open-paths.txt")
	env := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
		"ZV_FAKE_OPEN_PATH_LOG=" + openPathLogPath,
	}

	seen := make(map[string]bool)
	var wantSubcommandCalls, wantOpenPathCalls int
	for _, workflow := range workflowCatalog() {
		t.Run(workflow.Name, func(t *testing.T) {
			args := workflowRunCommandArgs(t, workflow)
			args = append(args, workflowRunSampleForwardedArgs(t, workflow, galleryPath)...)
			runZVBinaryWithEnvExpectFlowDryRunIncomplete(t, exe, tempDir, env, args...)
			seen[workflow.Name] = true
			switch {
			case workflow.RunArgs[0] == "gallery":
				wantOpenPathCalls++
			case !workflowDelegatesExternally(workflow):
			default:
				wantSubcommandCalls++
			}
		})
	}
	if got, want := len(seen), len(workflowCatalog()); got != want {
		t.Fatalf("executed workflows = %d, want %d: %#v", got, want, seen)
	}
	if got, want := len(readFakeSubcommandCalls(t, subcommandLogPath)), wantSubcommandCalls; got != want {
		t.Fatalf("subcommand calls = %d, want %d", got, want)
	}
	if got, want := len(readLines(t, openPathLogPath)), wantOpenPathCalls; got != want {
		t.Fatalf("open path calls = %d, want %d", got, want)
	}
}

func TestZVBinaryEveryWorkflowValidateCommandIsSideEffectFreeEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	subcommandLogPath := filepath.Join(tempDir, "validate-subcommands.jsonl")
	openPathLogPath := filepath.Join(tempDir, "validate-open-paths.txt")
	env := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
		"ZV_FAKE_OPEN_PATH_LOG=" + openPathLogPath,
	}

	for _, workflow := range workflowCatalog() {
		t.Run(workflow.Name, func(t *testing.T) {
			args := workflowValidateCommandArgs(t, workflow)
			args = append(args, "--format", "json")
			args = append(args, workflowRunSampleForwardedArgs(t, workflow, galleryPath)...)
			stdout, stderr := runZVBinarySplitWithEnv(t, exe, tempDir, env, args...)
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			var result workflowValidationResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal validation result: %v\n%s", err, stdout)
			}
			if !result.OK || result.Executed || result.Workflow != workflow.Name || result.Scope != "arguments" {
				t.Fatalf("validation result = %#v, want valid unexecuted %s preflight", result, workflow.Name)
			}
		})
	}

	assertPathDoesNotExist(t, subcommandLogPath)
	assertPathDoesNotExist(t, openPathLogPath)
}

func TestZVBinaryEveryWorkflowRunAcceptsEqualsRequiredFlagsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	writeSkillBody(t, tempDir, "alpha", strings.Join([]string{
		"---",
		"name: alpha",
		`description: "Alpha workflow"`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		"```",
		"",
	}, "\n"))
	writeWorkflowDocs(t, tempDir)
	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	writeFile(t, galleryPath, "<!doctype html><title>gallery</title>\n")

	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	subcommandLogPath := filepath.Join(tempDir, "equals-required-subcommands.jsonl")
	openPathLogPath := filepath.Join(tempDir, "equals-required-open-paths.txt")
	env := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
		"ZV_FAKE_OPEN_PATH_LOG=" + openPathLogPath,
	}

	var wantSubcommandCalls, wantOpenPathCalls int
	for _, workflow := range workflowCatalog() {
		required := requiredFlagsForRunArgs(workflow.RunArgs...)
		if len(required) == 0 {
			continue
		}
		t.Run(workflow.Name, func(t *testing.T) {
			args := workflowRunCommandArgs(t, workflow)
			forwarded := equalsRequiredFlags(t, workflowRunSampleForwardedArgs(t, workflow, galleryPath), required)
			args = append(args, forwarded...)
			runZVBinaryWithEnvExpectFlowDryRunIncomplete(t, exe, tempDir, env, args...)
			switch {
			case workflow.RunArgs[0] == "gallery":
				wantOpenPathCalls++
			case !workflowDelegatesExternally(workflow):
			default:
				wantSubcommandCalls++
			}
		})
	}
	if got, want := len(readFakeSubcommandCalls(t, subcommandLogPath)), wantSubcommandCalls; got != want {
		t.Fatalf("subcommand calls = %d, want %d", got, want)
	}
	if got, want := len(readLines(t, openPathLogPath)), wantOpenPathCalls; got != want {
		t.Fatalf("open path calls = %d, want %d", got, want)
	}
}

func TestZVBinaryEveryWorkflowRunRejectsForwardedArgsWithoutSeparatorEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	writeFile(t, galleryPath, "<!doctype html><title>gallery</title>\n")

	for _, workflow := range workflowCatalog() {
		t.Run(workflow.Name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, workflow.Name+"-missing-separator.jsonl")
			openPathLogPath := filepath.Join(tempDir, workflow.Name+"-missing-separator-open.txt")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
				"ZV_FAKE_OPEN_PATH_LOG=" + openPathLogPath,
			}

			args := workflowRunCommandArgs(t, workflow)
			args = append(args, workflowRunSampleArgsWithoutSeparator(t, workflow, galleryPath)...)
			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, `missing "--" separator before forwarded args for "workflows run"`) {
				t.Fatalf("stderr = %q, want missing separator error", stderr)
			}
			if !strings.Contains(stderr, workflowsRunUsage) {
				t.Fatalf("stderr = %q, want workflows run usage", stderr)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
			assertPathDoesNotExist(t, openPathLogPath)
		})
	}
}

func TestZVBinaryRepresentativeValidationErrorsAreExactEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	validDemoParseFlags := []string{"--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json"}
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "workflow run missing separator",
			args:       append([]string{"workflows", "run", "demo-parse"}, validDemoParseFlags...),
			wantStderr: "error: missing \"--\" separator before forwarded args for \"workflows run\"\n" + workflowsRunUsage,
		},
		{
			name:       "workflow run missing required flags",
			args:       []string{"workflows", "run", "demo-parse", "--"},
			wantStderr: "error: missing required flags --demo, --steamid, --out for \"demo parse\"\n" + workflowsRunUsage,
		},
		{
			name:       "workflow run unknown flag",
			args:       append(append([]string{"workflows", "run", "demo-parse", "--"}, validDemoParseFlags...), "--zv-unknown"),
			wantStderr: "error: unknown flag --zv-unknown for \"demo parse\"\n" + workflowsRunUsage,
		},
		{
			name:       "direct missing required flags",
			args:       []string{"demo", "parse"},
			wantStderr: "error: missing required flags --demo, --steamid, --out for \"demo parse\"\n",
		},
		{
			name:       "direct unexpected positional arg",
			args:       append(append([]string{"demo", "parse"}, validDemoParseFlags...), "tail"),
			wantStderr: "error: unexpected positional arg \"tail\" for \"demo parse\"; quote paths with spaces\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, tt.name+"-subcommands.jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, tt.args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got, want := stderr, tt.wantStderr; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
		})
	}
}

func TestZVBinaryCanonicalWorkflowFlagsAreScopedEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	valid := []struct {
		name string
		args []string
	}{
		{
			name: "demo parse optional flags",
			args: []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--segment-mode", "utility", "--rules", "rules.json", "--verbose"},
		},
		{
			name: "demo parse dry-run",
			args: []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--dry-run"},
		},
		{
			name: "utility audit optional format",
			args: []string{"utility", "audit", "--plan", "plan.json", "--lineup-catalog", "data/lineups", "--out", "audit.json", "--format", "json"},
		},
		{
			name: "shorts render optional flags",
			args: []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "--killplan", "plan.json", "--publish-dir", "publish", "--preset", "viral-60-clean", "--effects", "effects.lua", "--effects-preset", "viral-ultra-clean", "--music", "music.wav", "--rhythm", "rhythm.json", "--fps", "24", "--lineup-catalog", "lineups", "--segments", "seg-001", "--limit", "2", "--video-crf", "18", "--video-preset", "slow", "--ffmpeg", "ffmpeg.exe", "--ffprobe", "ffprobe.exe", "--hq-filters", "--audio-normalize", "--quality-checks", "--cover-sheets", "--temporal-smoothing", "--compile-segments", "--covers=false", "--no-covers", "--skip-existing", "--open-gallery", "--dry-run"},
		},
		{
			name: "shorts render standard preset",
			args: []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts-viral-60-clean", "--preset", "viral-60-clean", "--dry-run"},
		},
		{
			name: "analysis tactical data optional sample",
			args: []string{"analysis", "tactical-data", "--demo", "inferno.dem", "--out", "tactical.json", "--start", "1000", "--end", "2000", "--sample", "8"},
		},
		{
			name: "analysis viewer optional addr",
			args: []string{"analysis", "view", "--json", "analysis.json", "--addr", "127.0.0.1:0"},
		},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, "valid-"+strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr := runZVBinarySplitWithEnv(t, exe, tempDir, env, tt.args...)

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if got := len(readFakeSubcommandCalls(t, subcommandLogPath)); got != 1 {
				t.Fatalf("subcommand calls = %d, want 1", got)
			}
		})
	}

	invalid := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "demo parse rejects shorts boolean",
			args:       []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--open-gallery"},
			wantStderr: "error: unknown flag --open-gallery for \"demo parse\"\n",
		},
		{
			name:       "shorts render rejects demo players value flag",
			args:       []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "--contains", "zack"},
			wantStderr: "error: unknown flag --contains for \"shorts render\"\n",
		},
		{
			name:       "workflow run demo parse rejects record boolean",
			args:       []string{"workflows", "run", "demo-parse", "--", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--portrait-safe-killfeed"},
			wantStderr: "error: unknown flag --portrait-safe-killfeed for \"demo parse\"\n" + workflowsRunUsage,
		},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, "invalid-"+strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, tt.args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got, want := stderr, tt.wantStderr; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
		})
	}
}

func TestZVBinaryRecordAutoDetectsCaptureToolsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	valid := []struct {
		name           string
		args           []string
		env            []string
		wantExecutable string
		wantArgs       []string
	}{
		{
			name:           "direct dry run omits capture tools",
			args:           []string{"record", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run"},
		},
		{
			name:           "workflow dry run true omits capture tools",
			args:           []string{"workflows", "run", "record", "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=true"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=true"},
		},
		{
			name:           "direct capture tools",
			args:           []string{"record", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--hlae", "HLAE.exe", "--cs2", "cs2.exe"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--hlae", "HLAE.exe", "--cs2", "cs2.exe"},
		},
		{
			name: "direct auto detected capture tools",
			args: []string{"record", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording"},
			env: []string{
				`ZV_HLAE_PATH=C:\Detected\HLAE.exe`,
				`ZV_CS2_PATH=C:\Detected\cs2.exe`,
			},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--hlae", `C:\Detected\HLAE.exe`, "--cs2", `C:\Detected\cs2.exe`},
		},
		{
			name: "workflow auto detected capture tools",
			args: []string{"workflows", "run", "record", "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=false"},
			env: []string{
				`ZV_HLAE_PATH=C:\Detected\HLAE.exe`,
				`ZV_CS2_PATH=C:\Detected\cs2.exe`,
			},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=false", "--hlae", `C:\Detected\HLAE.exe`, "--cs2", `C:\Detected\cs2.exe`},
		},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, "valid-"+strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}
			env = append(env, tt.env...)

			stdout, stderr := runZVBinarySplitWithEnv(t, exe, tempDir, env, tt.args...)

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			calls := readFakeSubcommandCalls(t, subcommandLogPath)
			if got, want := len(calls), 1; got != want {
				t.Fatalf("subcommand calls = %d, want %d: %#v", got, want, calls)
			}
			if got, want := calls[0].Executable, tt.wantExecutable; got != want {
				t.Fatalf("executable = %q, want %q", got, want)
			}
			if got, want := strings.Join(calls[0].Args, "\x00"), strings.Join(tt.wantArgs, "\x00"); got != want {
				t.Fatalf("args = %#v, want %#v", calls[0].Args, tt.wantArgs)
			}
		})
	}

}

func TestZVBinaryRecordWorkflowDiscoveryDocumentsCaptureContractEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	showText, showTextStderr := runZVBinarySplit(t, exe, tempDir, "workflows", "show", "record")
	if showTextStderr != "" {
		t.Fatalf("workflows show record stderr = %q, want empty", showTextStderr)
	}
	for _, want := range []string{
		"command: zv record --killplan <plan.json> --demo <demo.dem> --out <recording-dir>",
		"run_command: zv workflows run record",
	} {
		if !strings.Contains(showText, want) {
			t.Fatalf("workflows show record = %q, want %q", showText, want)
		}
	}

	showJSON, showJSONStderr := runZVBinarySplit(t, exe, tempDir, "workflows", "show", "record", "--format", "json")
	if showJSONStderr != "" {
		t.Fatalf("workflows show record json stderr = %q, want empty", showJSONStderr)
	}
	var shown workflowInfo
	if err := json.Unmarshal([]byte(showJSON), &shown); err != nil {
		t.Fatalf("unmarshal workflows show record json: %v\n%s", err, showJSON)
	}
	if got, want := shown.Command, "zv record --killplan <plan.json> --demo <demo.dem> --out <recording-dir>"; got != want {
		t.Fatalf("record workflow command = %q, want %q", got, want)
	}
	if got, want := shown.RunCommand, "zv workflows run record"; got != want {
		t.Fatalf("record workflow run_command = %q, want %q", got, want)
	}

	listJSON, listJSONStderr := runZVBinarySplit(t, exe, tempDir, "workflows", "list", "--format", "json")
	if listJSONStderr != "" {
		t.Fatalf("workflows list json stderr = %q, want empty", listJSONStderr)
	}
	var listed []workflowInfo
	if err := json.Unmarshal([]byte(listJSON), &listed); err != nil {
		t.Fatalf("unmarshal workflows list json: %v\n%s", err, listJSON)
	}
	var listedRecord workflowInfo
	for _, workflow := range listed {
		if workflow.Name == "record" {
			listedRecord = workflow
			break
		}
	}
	if listedRecord.Name == "" {
		t.Fatalf("workflows list json did not include record workflow")
	}
	if got, want := listedRecord.Command, shown.Command; got != want {
		t.Fatalf("listed record command = %q, want show command %q", got, want)
	}
	if got, want := listedRecord.RunCommand, shown.RunCommand; got != want {
		t.Fatalf("listed record run_command = %q, want show run_command %q", got, want)
	}

	subcommandLogPath := filepath.Join(tempDir, "record-discovery-dry-run.jsonl")
	env := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
	}
	stdout, stderr := runZVBinarySplitWithEnv(t, exe, tempDir, env, "workflows", "run", "record", "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run")
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	calls := readFakeSubcommandCalls(t, subcommandLogPath)
	if got, want := len(calls), 1; got != want {
		t.Fatalf("subcommand calls = %d, want %d: %#v", got, want, calls)
	}
	if got, want := calls[0].Executable, executableName("zv-recorder"); got != want {
		t.Fatalf("executable = %q, want %q", got, want)
	}
	if got, want := strings.Join(calls[0].Args, "\x00"), strings.Join([]string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run"}, "\x00"); got != want {
		t.Fatalf("args = %#v, want dry-run record args", calls[0].Args)
	}

	captureLogPath := filepath.Join(tempDir, "record-discovery-capture.jsonl")
	captureEnv := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + captureLogPath,
	}
	discoveredArgs, ok := splitCommandFields(shown.RunCommand)
	if !ok {
		t.Fatalf("parse discovered run_command %q", shown.RunCommand)
	}
	discoveredArgs = append(discoveredArgs[1:], "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=false", "--hlae", "HLAE.exe", "--cs2", "cs2.exe")
	captureStdout, captureStderr := runZVBinarySplitWithEnv(t, exe, tempDir, captureEnv, discoveredArgs...)
	if captureStdout != "" {
		t.Fatalf("capture stdout = %q, want empty", captureStdout)
	}
	if captureStderr != "" {
		t.Fatalf("capture stderr = %q, want empty", captureStderr)
	}
	captureCalls := readFakeSubcommandCalls(t, captureLogPath)
	if got, want := len(captureCalls), 1; got != want {
		t.Fatalf("capture subcommand calls = %d, want %d: %#v", got, want, captureCalls)
	}
	if got, want := captureCalls[0].Executable, executableName("zv-recorder"); got != want {
		t.Fatalf("capture executable = %q, want %q", got, want)
	}
	if got, want := strings.Join(captureCalls[0].Args, "\x00"), strings.Join([]string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=false", "--hlae", "HLAE.exe", "--cs2", "cs2.exe"}, "\x00"); got != want {
		t.Fatalf("capture args = %#v, want capture record args", captureCalls[0].Args)
	}
}

func TestZVBinaryCanonicalWorkflowOptionalValueFlagsRequireValuesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "direct demo players missing contains",
			args:       []string{"demo", "players", "--demo", "inferno.dem", "--contains"},
			wantStderr: "error: missing value for flag --contains for \"demo players\"\n",
		},
		{
			name:       "workflow demo players missing contains",
			args:       []string{"workflows", "run", "demo-players", "--", "--demo", "inferno.dem", "--contains"},
			wantStderr: "error: missing value for flag --contains for \"demo players\"\n" + workflowsRunUsage,
		},
		{
			name:       "direct record empty timeout",
			args:       []string{"record", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run", "--timeout="},
			wantStderr: "error: missing value for flag --timeout for \"record\"\n",
		},
		{
			name:       "workflow compose final missing ffmpeg",
			args:       []string{"workflows", "run", "compose-final", "--", "--recording-result", "recording-result.json", "--out", "final.mp4", "--dry-run", "--ffmpeg"},
			wantStderr: "error: missing value for flag --ffmpeg for \"compose final\"\n" + workflowsRunUsage,
		},
		{
			name:       "direct shorts render empty limit",
			args:       []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "--limit="},
			wantStderr: "error: missing value for flag --limit for \"shorts render\"\n",
		},
		{
			name:       "workflow shorts render missing effects",
			args:       []string{"workflows", "run", "shorts-render", "--", "--recording-result", "recording-result.json", "--out", "shorts", "--effects"},
			wantStderr: "error: missing value for flag --effects for \"shorts render\"\n" + workflowsRunUsage,
		},
		{
			name:       "direct analysis tactical data empty sample",
			args:       []string{"analysis", "tactical-data", "--demo", "inferno.dem", "--out", "tactical.json", "--start", "1000", "--end", "2000", "--sample", ""},
			wantStderr: "error: missing value for flag --sample for \"analysis tactical-data\"\n",
		},
		{
			name:       "workflow analysis viewer missing addr",
			args:       []string{"workflows", "run", "analysis-viewer", "--", "--json", "analysis.json", "--addr"},
			wantStderr: "error: missing value for flag --addr for \"analysis view\"\n" + workflowsRunUsage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, tt.args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got, want := stderr, tt.wantStderr; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
		})
	}
}

func TestZVBinaryCanonicalWorkflowAllowsDashPrefixedValuesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	tests := []struct {
		name           string
		args           []string
		wantExecutable string
		wantArgs       []string
	}{
		{
			name:           "direct demo parse dash prefixed demo path",
			args:           []string{"demo", "parse", "--demo", "-inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json"},
			wantExecutable: executableName("zv-parser"),
			wantArgs:       []string{"parse", "--demo", "-inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json"},
		},
		{
			name:           "workflow record negative numeric option",
			args:           []string{"workflows", "run", "record", "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run", "--video-crf", "-1"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run", "--video-crf", "-1"},
		},
		{
			name:           "workflow analysis tactical data negative required tick",
			args:           []string{"workflows", "run", "analysis-tactical-data", "--", "--demo", "inferno.dem", "--out", "tactical.json", "--start", "-1", "--end", "2000"},
			wantExecutable: executableName("zv-tactical-data"),
			wantArgs:       []string{"--demo", "inferno.dem", "--out", "tactical.json", "--start", "-1", "--end", "2000"},
		},
		{
			name:           "direct shorts render negative numeric option",
			args:           []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "--video-crf", "-1"},
			wantExecutable: executableName("zv-editor"),
			wantArgs:       []string{"--recording-result", "recording-result.json", "--out", "shorts", "--video-crf", "-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr := runZVBinarySplitWithEnv(t, exe, tempDir, env, tt.args...)

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			calls := readFakeSubcommandCalls(t, subcommandLogPath)
			if got, want := len(calls), 1; got != want {
				t.Fatalf("subcommand calls = %d, want %d: %#v", got, want, calls)
			}
			if got, want := calls[0].Executable, tt.wantExecutable; got != want {
				t.Fatalf("executable = %q, want %q", got, want)
			}
			if got, want := strings.Join(calls[0].Args, "\x00"), strings.Join(tt.wantArgs, "\x00"); got != want {
				t.Fatalf("args = %#v, want %#v", calls[0].Args, tt.wantArgs)
			}
		})
	}
}

func TestZVBinaryCanonicalWorkflowRejectsSingleDashFlagsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "direct demo parse single dash required flag",
			args:       []string{"demo", "parse", "-demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json"},
			wantStderr: "error: unknown flag -demo for \"demo parse\"\n",
		},
		{
			name:       "direct shorts render single dash option",
			args:       []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "-limit", "1"},
			wantStderr: "error: unknown flag -limit for \"shorts render\"\n",
		},
		{
			name:       "workflow record single dash dry run",
			args:       []string{"workflows", "run", "record", "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "-dry-run"},
			wantStderr: "error: unknown flag -dry-run for \"record\"\n" + workflowsRunUsage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, tt.args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got, want := stderr, tt.wantStderr; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
		})
	}
}

func TestZVBinaryCanonicalWorkflowBooleanEqualsValuesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	valid := []struct {
		name           string
		args           []string
		wantExecutable string
		wantArgs       []string
	}{
		{
			name:           "direct demo parse verbose false",
			args:           []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--verbose=false"},
			wantExecutable: executableName("zv-parser"),
			wantArgs:       []string{"parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--verbose=false"},
		},
		{
			name:           "workflow record dry run false",
			args:           []string{"workflows", "run", "record", "--", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=false", "--hlae", "HLAE.exe", "--cs2", "cs2.exe"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=false", "--hlae", "HLAE.exe", "--cs2", "cs2.exe"},
		},
		{
			name:           "direct shorts render open gallery zero",
			args:           []string{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "--open-gallery=0", "--covers=True"},
			wantExecutable: executableName("zv-editor"),
			wantArgs:       []string{"--recording-result", "recording-result.json", "--out", "shorts", "--open-gallery=0", "--covers=True"},
		},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, "valid-"+strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr := runZVBinarySplitWithEnv(t, exe, tempDir, env, tt.args...)

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			calls := readFakeSubcommandCalls(t, subcommandLogPath)
			if got, want := len(calls), 1; got != want {
				t.Fatalf("subcommand calls = %d, want %d: %#v", got, want, calls)
			}
			if got, want := calls[0].Executable, tt.wantExecutable; got != want {
				t.Fatalf("executable = %q, want %q", got, want)
			}
			if got, want := strings.Join(calls[0].Args, "\x00"), strings.Join(tt.wantArgs, "\x00"); got != want {
				t.Fatalf("args = %#v, want %#v", calls[0].Args, tt.wantArgs)
			}
		})
	}

	invalid := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "direct demo parse empty verbose",
			args:       []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--verbose="},
			wantStderr: "error: invalid boolean value \"\" for flag --verbose for \"demo parse\"\n",
		},
		{
			name:       "direct record invalid dry run",
			args:       []string{"record", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run=maybe"},
			wantStderr: "error: invalid boolean value \"maybe\" for flag --dry-run for \"record\"\n",
		},
		{
			name:       "workflow shorts render invalid open gallery",
			args:       []string{"workflows", "run", "shorts-render", "--", "--recording-result", "recording-result.json", "--out", "shorts", "--open-gallery=maybe"},
			wantStderr: "error: invalid boolean value \"maybe\" for flag --open-gallery for \"shorts render\"\n" + workflowsRunUsage,
		},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, "invalid-"+strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, tt.args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got, want := stderr, tt.wantStderr; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
		})
	}
}

func TestZVBinaryCanonicalWorkflowBooleanFlagsRejectSeparateValuesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "direct demo parse verbose false",
			args:       []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "plan.json", "--verbose", "false"},
			wantStderr: "error: boolean flag --verbose for \"demo parse\" does not take separate value \"false\"; use --verbose=false\n",
		},
		{
			name:       "direct record dry run false",
			args:       []string{"record", "--killplan", "plan.json", "--demo", "inferno.dem", "--out", "recording", "--dry-run", "false"},
			wantStderr: "error: boolean flag --dry-run for \"record\" does not take separate value \"false\"; use --dry-run=false\n",
		},
		{
			name:       "workflow compose final dry run false",
			args:       []string{"workflows", "run", "compose-final", "--", "--recording-result", "recording-result.json", "--out", "final.mp4", "--dry-run", "false"},
			wantStderr: "error: boolean flag --dry-run for \"compose final\" does not take separate value \"false\"; use --dry-run=false\n" + workflowsRunUsage,
		},
		{
			name:       "workflow shorts render open gallery false",
			args:       []string{"workflows", "run", "shorts-render", "--", "--recording-result", "recording-result.json", "--out", "shorts", "--open-gallery", "false"},
			wantStderr: "error: boolean flag --open-gallery for \"shorts render\" does not take separate value \"false\"; use --open-gallery=false\n" + workflowsRunUsage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subcommandLogPath := filepath.Join(tempDir, strings.ReplaceAll(tt.name, " ", "-")+".jsonl")
			env := []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
			}

			stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, tt.args...)

			if got, want := code, exitInvalidArgs; got != want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got, want := stderr, tt.wantStderr; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			assertPathDoesNotExist(t, subcommandLogPath)
		})
	}
}

func TestZVBinaryEveryDirectWorkflowAcceptsEqualsRequiredFlagsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	writeSkillBody(t, tempDir, "alpha", strings.Join([]string{
		"---",
		"name: alpha",
		`description: "Alpha workflow"`,
		"---",
		"",
		"```powershell",
		`.\bin\zv.exe workflows run demo-parse -- --demo demo.dem --steamid 76561198000000000 --out plan.json`,
		"```",
		"",
	}, "\n"))
	writeWorkflowDocs(t, tempDir)
	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	writeFile(t, galleryPath, "<!doctype html><title>gallery</title>\n")

	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	subcommandLogPath := filepath.Join(tempDir, "direct-equals-required-subcommands.jsonl")
	openPathLogPath := filepath.Join(tempDir, "direct-equals-required-open-paths.txt")
	env := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
		"ZV_FAKE_OPEN_PATH_LOG=" + openPathLogPath,
	}

	var wantSubcommandCalls, wantOpenPathCalls int
	for _, workflow := range workflowCatalog() {
		required := requiredFlagsForRunArgs(workflow.RunArgs...)
		if len(required) == 0 {
			continue
		}
		t.Run(workflow.Name, func(t *testing.T) {
			args := equalsRequiredFlags(t, workflowDirectSampleArgs(t, workflow, galleryPath), required)
			runZVBinaryWithEnvExpectFlowDryRunIncomplete(t, exe, tempDir, env, args...)
			switch {
			case workflow.RunArgs[0] == "gallery":
				wantOpenPathCalls++
			case !workflowDelegatesExternally(workflow):
			default:
				wantSubcommandCalls++
			}
		})
	}
	if got, want := len(readFakeSubcommandCalls(t, subcommandLogPath)), wantSubcommandCalls; got != want {
		t.Fatalf("subcommand calls = %d, want %d", got, want)
	}
	if got, want := len(readLines(t, openPathLogPath)), wantOpenPathCalls; got != want {
		t.Fatalf("open path calls = %d, want %d", got, want)
	}
}

func TestZVBinaryEveryWorkflowRejectsInvalidRequiredArgsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	writeFile(t, galleryPath, "<!doctype html><title>gallery</title>\n")

	// bare is the command without forwarded args; sample is the valid catalog sample.
	type invocation struct {
		bare, sample []string
		run          bool
	}
	mutations := []struct {
		name    string
		runOnly bool
		mutate  func(t *testing.T, call invocation, flag string) []string
		want    func(flag string) string
	}{
		{
			name:   "missing required args",
			mutate: func(_ *testing.T, call invocation, _ string) []string { return call.bare },
			want:   func(string) string { return "missing required flag" },
		},
		{
			name:    "empty forwarded required args",
			runOnly: true,
			mutate: func(_ *testing.T, call invocation, _ string) []string {
				return append(append([]string(nil), call.bare...), "--")
			},
			want: func(string) string { return "missing required flag" },
		},
		{
			name: "empty equals required flag",
			mutate: func(t *testing.T, call invocation, flag string) []string {
				return emptyEqualsRequiredFlag(t, call.sample, flag)
			},
			want: func(string) string { return "missing required flag" },
		},
		{
			name: "empty separate required flag",
			mutate: func(t *testing.T, call invocation, flag string) []string {
				return emptySeparateRequiredFlag(t, call.sample, flag)
			},
			want: func(string) string { return "missing required flag" },
		},
		{
			name: "duplicate required flag",
			mutate: func(t *testing.T, call invocation, flag string) []string {
				return duplicateFlagValue(t, call.sample, flag)
			},
			want: func(flag string) string { return "duplicate flag " + flag },
		},
		{
			name: "unexpected positional arg",
			mutate: func(_ *testing.T, call invocation, _ string) []string {
				return append(append([]string(nil), call.sample...), "unquoted path tail")
			},
			want: func(string) string { return `unexpected positional arg "unquoted path tail"` },
		},
		{
			name: "unknown flag",
			mutate: func(_ *testing.T, call invocation, _ string) []string {
				return append(append([]string(nil), call.sample...), "--zv-unknown")
			},
			want: func(string) string { return "unknown flag --zv-unknown" },
		},
		{
			name: "unknown equals flag",
			mutate: func(_ *testing.T, call invocation, _ string) []string {
				return append(append([]string(nil), call.sample...), "--zv-unknown=value")
			},
			want: func(string) string { return "unknown flag --zv-unknown" },
		},
	}

	for _, workflow := range workflowCatalog() {
		required := requiredFlagsForRunArgs(workflow.RunArgs...)
		if len(required) == 0 {
			continue
		}
		runArgs := workflowRunCommandArgs(t, workflow)
		invocations := map[string]invocation{
			"workflows run": {
				bare:   runArgs,
				sample: append(append([]string(nil), runArgs...), workflowRunSampleForwardedArgs(t, workflow, galleryPath)...),
				run:    true,
			},
			"direct": {
				bare:   append([]string(nil), workflow.RunArgs...),
				sample: workflowDirectSampleArgs(t, workflow, galleryPath),
			},
		}
		for mode, call := range invocations {
			for _, mutation := range mutations {
				if mutation.runOnly && !call.run {
					continue
				}
				t.Run(workflow.Name+"/"+mode+"/"+mutation.name, func(t *testing.T) {
					logPrefix := filepath.Join(tempDir, strings.ReplaceAll(workflow.Name+"-"+mode+"-"+mutation.name, " ", "-"))
					subcommandLogPath := logPrefix + ".jsonl"
					openPathLogPath := logPrefix + "-open.txt"
					env := []string{
						"ZV_FAKE_SUBCOMMAND=1",
						"ZV_FAKE_SUBCOMMAND_LOG=" + subcommandLogPath,
						"ZV_FAKE_OPEN_PATH_LOG=" + openPathLogPath,
					}

					args := mutation.mutate(t, call, required[0])
					stdout, stderr, code := runZVBinaryFailureSplitWithEnv(t, exe, tempDir, env, args...)

					if got, want := code, exitInvalidArgs; got != want {
						t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, want, stdout, stderr)
					}
					if stdout != "" {
						t.Fatalf("stdout = %q, want empty", stdout)
					}
					if want := mutation.want(required[0]); !strings.Contains(stderr, want) {
						t.Fatalf("stderr = %q, want %q", stderr, want)
					}
					if call.run && !strings.Contains(stderr, workflowsRunUsage) {
						t.Fatalf("stderr = %q, want workflows run usage", stderr)
					}
					assertPathDoesNotExist(t, subcommandLogPath)
					assertPathDoesNotExist(t, openPathLogPath)
				})
			}
		}
	}
}

func TestZVBinaryWorkflowListAndShowJSONMatchCatalogEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)

	listJSON, stderr := runZVBinarySplit(t, exe, tempDir, "workflows", "list", "--format", "json")
	if stderr != "" {
		t.Fatalf("workflows list json wrote stderr %q", stderr)
	}
	var listed []workflowInfo
	if err := json.Unmarshal([]byte(listJSON), &listed); err != nil {
		t.Fatalf("unmarshal workflows list json: %v\n%s", err, listJSON)
	}

	catalog := workflowCatalog()
	if got, want := len(listed), len(catalog); got != want {
		t.Fatalf("workflows list json count = %d, want %d", got, want)
	}
	for i, catalogWorkflow := range catalog {
		listedWorkflow := listed[i]
		assertWorkflowDiscoveryMatches(t, "workflows list json", listedWorkflow, catalogWorkflow)

		showJSON, stderr := runZVBinarySplit(t, exe, tempDir, "workflows", "show", catalogWorkflow.Name, "--format", "json")
		if stderr != "" {
			t.Fatalf("workflows show json for %s wrote stderr %q", catalogWorkflow.Name, stderr)
		}
		var shown workflowInfo
		if err := json.Unmarshal([]byte(showJSON), &shown); err != nil {
			t.Fatalf("unmarshal workflows show json for %s: %v\n%s", catalogWorkflow.Name, err, showJSON)
		}
		assertWorkflowDiscoveryMatches(t, "workflows show json", shown, catalogWorkflow)

		if listedWorkflow.Name != shown.Name ||
			listedWorkflow.Description != shown.Description ||
			listedWorkflow.Command != shown.Command ||
			listedWorkflow.RunCommand != shown.RunCommand {
			t.Fatalf("workflow discovery mismatch for %s\nlist: %#v\nshow: %#v", catalogWorkflow.Name, listedWorkflow, shown)
		}
	}
}

func TestZVBinaryCanonicalWorkflowDelegatesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	logPath := filepath.Join(tempDir, "calls.jsonl")
	env := []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + logPath,
	}
	tests := []struct {
		name           string
		args           []string
		wantExecutable string
		wantArgs       []string
	}{
		{
			name:           "demo parse",
			args:           []string{"demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--segment-mode", "utility", "--out", "run/plan.json"},
			wantExecutable: executableName("zv-parser"),
			wantArgs:       []string{"parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--segment-mode", "utility", "--out", "run/plan.json"},
		},
		{
			name:           "demo players",
			args:           []string{"demo", "players", "--demo", "inferno.dem", "--contains", "maaryy"},
			wantExecutable: executableName("zv-demo-players"),
			wantArgs:       []string{"--demo", "inferno.dem", "--contains", "maaryy"},
		},
		{
			name:           "utility audit",
			args:           []string{"utility", "audit", "--plan", "run/plan.json", "--lineup-catalog", "data/lineups", "--out", "run/utility-audit.csv"},
			wantExecutable: executableName("zv-parser"),
			wantArgs:       []string{"utility-audit", "--plan", "run/plan.json", "--lineup-catalog", "data/lineups", "--out", "run/utility-audit.csv"},
		},
		{
			name:           "record",
			args:           []string{"record", "--killplan", "run/plan.json", "--demo", "inferno.dem", "--out", "run/recording", "--dry-run"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "run/plan.json", "--demo", "inferno.dem", "--out", "run/recording", "--dry-run"},
		},
		{
			name:           "compose final",
			args:           []string{"compose", "final", "--recording-result", "run/recording/recording-result.json", "--out", "run/final.mp4", "--dry-run"},
			wantExecutable: executableName("zv-composer"),
			wantArgs:       []string{"--recording-result", "run/recording/recording-result.json", "--out", "run/final.mp4", "--dry-run"},
		},
		{
			name:           "shorts render",
			args:           []string{"shorts", "render", "--recording-result", "run/recording/recording-result.json", "--killplan", "run/plan.json", "--out", "run/shorts", "--preset", "viral-60-clean"},
			wantExecutable: executableName("zv-editor"),
			wantArgs:       []string{"--recording-result", "run/recording/recording-result.json", "--killplan", "run/plan.json", "--out", "run/shorts", "--preset", "viral-60-clean"},
		},
		{
			name:           "analysis tactical data",
			args:           []string{"analysis", "tactical-data", "--demo", "inferno.dem", "--out", "run/tactical.json", "--start", "1000", "--end", "2000"},
			wantExecutable: executableName("zv-tactical-data"),
			wantArgs:       []string{"--demo", "inferno.dem", "--out", "run/tactical.json", "--start", "1000", "--end", "2000"},
		},
		{
			name:           "analysis view",
			args:           []string{"analysis", "view", "--json", "run/analysis.json", "--addr", "127.0.0.1:0"},
			wantExecutable: executableName("zv-analysis-viewer"),
			wantArgs:       []string{"--json", "run/analysis.json", "--addr", "127.0.0.1:0"},
		},
		{
			name:           "serve",
			args:           []string{"serve"},
			wantExecutable: executableName("zv-orchestrator"),
			wantArgs:       nil,
		},
		{
			name:           "workflow run demo parse",
			args:           []string{"workflows", "run", "demo-parse", "--", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "run/plan.json"},
			wantExecutable: executableName("zv-parser"),
			wantArgs:       []string{"parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "run/plan.json"},
		},
		{
			name:           "workflow run demo players",
			args:           []string{"workflows", "run", "demo-players", "--", "--demo", "inferno.dem"},
			wantExecutable: executableName("zv-demo-players"),
			wantArgs:       []string{"--demo", "inferno.dem"},
		},
		{
			name:           "workflow run utility audit",
			args:           []string{"workflows", "run", "utility-audit", "--", "--plan", "run/plan.json", "--lineup-catalog", "data/lineups", "--out", "run/utility-audit.csv"},
			wantExecutable: executableName("zv-parser"),
			wantArgs:       []string{"utility-audit", "--plan", "run/plan.json", "--lineup-catalog", "data/lineups", "--out", "run/utility-audit.csv"},
		},
		{
			name:           "workflow run record",
			args:           []string{"workflows", "run", "record", "--", "--killplan", "run/plan.json", "--demo", "inferno.dem", "--out", "run/recording", "--dry-run"},
			wantExecutable: executableName("zv-recorder"),
			wantArgs:       []string{"--killplan", "run/plan.json", "--demo", "inferno.dem", "--out", "run/recording", "--dry-run"},
		},
		{
			name:           "workflow run compose final",
			args:           []string{"workflows", "run", "compose-final", "--", "--recording-result", "run/recording/recording-result.json", "--out", "run/final.mp4", "--dry-run"},
			wantExecutable: executableName("zv-composer"),
			wantArgs:       []string{"--recording-result", "run/recording/recording-result.json", "--out", "run/final.mp4", "--dry-run"},
		},
		{
			name:           "workflow run shorts render",
			args:           []string{"workflows", "run", "shorts-render", "--", "--recording-result", "run/recording/recording-result.json", "--out", "run/shorts"},
			wantExecutable: executableName("zv-editor"),
			wantArgs:       []string{"--recording-result", "run/recording/recording-result.json", "--out", "run/shorts"},
		},
		{
			name:           "workflow run analysis tactical data",
			args:           []string{"workflows", "run", "analysis-tactical-data", "--", "--demo", "inferno.dem", "--out", "run/tactical.json", "--start", "1000", "--end", "2000"},
			wantExecutable: executableName("zv-tactical-data"),
			wantArgs:       []string{"--demo", "inferno.dem", "--out", "run/tactical.json", "--start", "1000", "--end", "2000"},
		},
		{
			name:           "workflow run analysis view",
			args:           []string{"workflows", "run", "analysis-viewer", "--", "--json", "run/analysis.json"},
			wantExecutable: executableName("zv-analysis-viewer"),
			wantArgs:       []string{"--json", "run/analysis.json"},
		},
		{
			name:           "workflow run serve",
			args:           []string{"workflows", "run", "serve"},
			wantExecutable: executableName("zv-orchestrator"),
			wantArgs:       nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runZVBinaryWithEnv(t, exe, tempDir, env, tt.args...)
		})
	}

	calls := readFakeSubcommandCalls(t, logPath)
	if got, want := len(calls), len(tests); got != want {
		t.Fatalf("calls len = %d, want %d: %#v", got, want, calls)
	}
	for i, tt := range tests {
		call := calls[i]
		if got, want := call.Executable, tt.wantExecutable; got != want {
			t.Fatalf("call %d executable = %q, want %q", i, got, want)
		}
		if got, want := strings.Join(call.Args, "\x00"), strings.Join(tt.wantArgs, "\x00"); got != want {
			t.Fatalf("call %d args = %#v, want %#v", i, call.Args, tt.wantArgs)
		}
	}
}

func TestZVBinaryWorkflowRunsMatchDirectDelegationEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	galleryPath := filepath.Join(tempDir, "gallery", "index.html")
	writeFile(t, galleryPath, "<!doctype html><title>gallery</title>\n")

	for _, workflow := range workflowCatalog() {
		if !workflowDelegatesExternally(workflow) {
			continue
		}
		t.Run(workflow.Name, func(t *testing.T) {
			directLogPath := filepath.Join(tempDir, workflow.Name+"-direct.jsonl")
			runLogPath := filepath.Join(tempDir, workflow.Name+"-run.jsonl")

			directArgs := workflowDirectSampleArgs(t, workflow, galleryPath)
			runZVBinaryWithEnv(t, exe, tempDir, []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + directLogPath,
			}, directArgs...)

			runArgs := workflowRunCommandArgs(t, workflow)
			runArgs = append(runArgs, workflowRunSampleForwardedArgs(t, workflow, galleryPath)...)
			runZVBinaryWithEnv(t, exe, tempDir, []string{
				"ZV_FAKE_SUBCOMMAND=1",
				"ZV_FAKE_SUBCOMMAND_LOG=" + runLogPath,
			}, runArgs...)

			directCalls := readFakeSubcommandCalls(t, directLogPath)
			runCalls := readFakeSubcommandCalls(t, runLogPath)
			if got, want := len(directCalls), 1; got != want {
				t.Fatalf("direct calls len = %d, want %d: %#v", got, want, directCalls)
			}
			if got, want := len(runCalls), 1; got != want {
				t.Fatalf("workflow run calls len = %d, want %d: %#v", got, want, runCalls)
			}
			if got, want := runCalls[0].Executable, directCalls[0].Executable; got != want {
				t.Fatalf("workflow run executable = %q, want direct executable %q", got, want)
			}
			if got, want := strings.Join(runCalls[0].Args, "\x00"), strings.Join(directCalls[0].Args, "\x00"); got != want {
				t.Fatalf("workflow run args = %#v, want direct args %#v", runCalls[0].Args, directCalls[0].Args)
			}
		})
	}
}

func TestZVBinaryWorkflowRunPreservesDelegatedStdioEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	input := "demo stdin payload\n"
	directStdout, directStderr := runZVBinarySplitWithEnvAndInput(t, exe, tempDir, []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + filepath.Join(tempDir, "direct-stdio.jsonl"),
		"ZV_FAKE_SUBCOMMAND_ECHO_STDIO=1",
	}, input, "demo", "parse", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "run/plan.json")
	runStdout, runStderr := runZVBinarySplitWithEnvAndInput(t, exe, tempDir, []string{
		"ZV_FAKE_SUBCOMMAND=1",
		"ZV_FAKE_SUBCOMMAND_LOG=" + filepath.Join(tempDir, "run-stdio.jsonl"),
		"ZV_FAKE_SUBCOMMAND_ECHO_STDIO=1",
	}, input, "workflows", "run", "demo-parse", "--", "--demo", "inferno.dem", "--steamid", "76561198000000000", "--out", "run/plan.json")

	if got, want := runStdout, directStdout; got != want {
		t.Fatalf("workflow run stdout = %q, want direct stdout %q", got, want)
	}
	if got, want := runStderr, directStderr; got != want {
		t.Fatalf("workflow run stderr = %q, want direct stderr %q", got, want)
	}
	if !strings.Contains(runStdout, input) {
		t.Fatalf("workflow run stdout = %q, want echoed stdin %q", runStdout, input)
	}
	if !strings.Contains(runStderr, executableName("zv-parser")) {
		t.Fatalf("workflow run stderr = %q, want delegated stderr marker", runStderr)
	}
}

func TestZVBinaryWorkflowHelpMatchesDirectDelegationEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	exe := buildZVBinary(t, tempDir)
	installFakeDelegatedSubcommands(t, filepath.Dir(exe))

	for i, alias := range []string{"--help", "-h", "help"} {
		for _, workflow := range workflowCatalog() {
			if !workflowHelpDelegatesExternally(workflow) {
				continue
			}
			t.Run(workflow.Name+"/"+alias, func(t *testing.T) {
				logPrefix := filepath.Join(tempDir, fmt.Sprintf("%s-alias%d", workflow.Name, i))
				directLogPath := logPrefix + "-direct-help.jsonl"
				runLogPath := logPrefix + "-run-help.jsonl"

				directArgs := append([]string(nil), workflow.RunArgs...)
				directArgs = append(directArgs, alias)
				runZVBinaryWithEnv(t, exe, tempDir, []string{
					"ZV_FAKE_SUBCOMMAND=1",
					"ZV_FAKE_SUBCOMMAND_LOG=" + directLogPath,
				}, directArgs...)

				runArgs := workflowRunCommandArgs(t, workflow)
				runArgs = append(runArgs, "--", alias)
				runZVBinaryWithEnv(t, exe, tempDir, []string{
					"ZV_FAKE_SUBCOMMAND=1",
					"ZV_FAKE_SUBCOMMAND_LOG=" + runLogPath,
				}, runArgs...)

				directCalls := readFakeSubcommandCalls(t, directLogPath)
				runCalls := readFakeSubcommandCalls(t, runLogPath)
				if got, want := len(directCalls), 1; got != want {
					t.Fatalf("direct help calls len = %d, want %d: %#v", got, want, directCalls)
				}
				if got, want := len(runCalls), 1; got != want {
					t.Fatalf("workflow help calls len = %d, want %d: %#v", got, want, runCalls)
				}
				if got, want := runCalls[0].Executable, directCalls[0].Executable; got != want {
					t.Fatalf("workflow help executable = %q, want direct executable %q", got, want)
				}
				if got, want := strings.Join(runCalls[0].Args, "\x00"), strings.Join(directCalls[0].Args, "\x00"); got != want {
					t.Fatalf("workflow help args = %#v, want direct args %#v", runCalls[0].Args, directCalls[0].Args)
				}
			})
		}
	}
}
