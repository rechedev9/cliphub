package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/rechedev9/cliphub/internal/verify"
)

func runVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, verifyUsage)
		return exitInvalidArgs
	}
	if isHelp(args[0]) {
		fmt.Fprint(stdout, verifyUsage)
		return exitSuccess
	}
	switch args[0] {
	case "doctor":
		return runVerifyDoctor(args[1:], stdout, stderr)
	case "features":
		return runVerifyFeatures(args[1:], stdout, stderr)
	case "http":
		return runVerifyHTTP(args[1:], stdout, stderr)
	case "gates":
		return runVerifyGates(args[1:], stdout, stderr)
	case "prove":
		return runVerifyProve(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown verify command %q\n%s", args[0], verifyUsage)
		return exitInvalidArgs
	}
}

func runVerifyDoctor(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, verifyDoctorUsage)
		return exitSuccess
	}
	format, flags, err := parseVerifyFlags("doctor", args, []string{"--dry-run", "--user-data"})
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyDoctorUsage, exitInvalidArgs)
	}
	if flags.URL != "" || flags.Feature != "" || flags.Run || flags.JobID != "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("unexpected extra args for %q", "verify doctor"), verifyDoctorUsage, exitInvalidArgs)
	}
	root, err := verify.FindRepoRoot()
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyDoctorUsage, exitUnexpected)
	}
	report := verify.Doctor(verify.DoctorOptions{
		Root:     root,
		DryRun:   flags.DryRun,
		UserData: flags.UserData,
	})
	if err := writeVerifyResult(stdout, stderr, format, report); err != nil {
		return exitUnexpected
	}
	if report.Closed || !report.OK {
		return exitInvalidArgs
	}
	return exitSuccess
}

func runVerifyFeatures(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, verifyFeaturesUsage)
		return exitSuccess
	}
	format, flags, err := parseVerifyFlags("features", args, []string{"--feature"})
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyFeaturesUsage, exitInvalidArgs)
	}
	root, err := verify.FindRepoRoot()
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyFeaturesUsage, exitUnexpected)
	}
	report := verify.InspectFeatureMap(root)
	if flags.Feature != "" {
		filtered := report
		filtered.Features = nil
		for _, feature := range report.Features {
			if feature.ID == flags.Feature {
				filtered.Features = []verify.FeatureMapStatus{feature}
				filtered.Issues = append([]string(nil), feature.Issues...)
				filtered.OK = feature.CheapOK && report.IndexPresent
				break
			}
		}
		if len(filtered.Features) == 0 {
			return writeCommandError(args, stdout, stderr, fmt.Errorf("unknown feature %q", flags.Feature), verifyFeaturesUsage, exitInvalidArgs)
		}
		report = filtered
	}
	if err := writeVerifyResult(stdout, stderr, format, report); err != nil {
		return exitUnexpected
	}
	if !report.OK {
		return exitInvalidArgs
	}
	return exitSuccess
}

func runVerifyHTTP(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, verifyHTTPUsage)
		return exitSuccess
	}
	format, flags, err := parseVerifyFlags("http", args, []string{"--url"})
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyHTTPUsage, exitInvalidArgs)
	}
	base := flags.URL
	if base == "" {
		base = verify.DefaultOrchestratorURL
	}
	report := verify.ProbeOrchestrator(base)
	if err := writeVerifyResult(stdout, stderr, format, report); err != nil {
		return exitUnexpected
	}
	if report.Status == verify.HTTPStatusMismatch || report.Status == verify.HTTPStatusRejected {
		return exitInvalidArgs
	}
	return exitSuccess
}

func runVerifyGates(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, verifyGatesUsage)
		return exitSuccess
	}
	format, flags, err := parseVerifyFlags("gates", args, []string{"--dry-run", "--run"})
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyGatesUsage, exitInvalidArgs)
	}
	if flags.Run && !flags.DryRun {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("zv verify gates does not become a second CI runner; pass --dry-run to print the existing hosted commands"), verifyGatesUsage, exitInvalidArgs)
	}
	report := verify.InspectGates(true)
	if err := writeVerifyResult(stdout, stderr, format, report); err != nil {
		return exitUnexpected
	}
	return exitSuccess
}

func runVerifyProve(args []string, stdout, stderr io.Writer) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, verifyProveUsage)
		return exitSuccess
	}
	format, flags, err := parseVerifyFlags("prove", args, []string{"--feature", "--dry-run", "--job-id", "--user-data"})
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyProveUsage, exitInvalidArgs)
	}
	if flags.Feature == "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf(`"verify prove" requires --feature`), verifyProveUsage, exitInvalidArgs)
	}
	root, err := verify.FindRepoRoot()
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyProveUsage, exitUnexpected)
	}
	report, err := verify.Prove(verify.ProveOptions{
		Root:     root,
		Feature:  flags.Feature,
		JobID:    flags.JobID,
		DryRun:   flags.DryRun,
		UserData: flags.UserData,
	})
	if err != nil {
		return writeCommandError(args, stdout, stderr, err, verifyProveUsage, exitInvalidArgs)
	}
	if err := writeVerifyResult(stdout, stderr, format, report); err != nil {
		return exitUnexpected
	}
	if report.Closed || !report.OK {
		return exitInvalidArgs
	}
	return exitSuccess
}

type verifyFlags struct {
	URL      string
	Feature  string
	JobID    string
	UserData string
	DryRun   bool
	Run      bool
}

func parseVerifyFlags(command string, args []string, allowed []string) (string, verifyFlags, error) {
	format, rest, err := parseFormatArgs(args)
	if err != nil {
		return "", verifyFlags{}, err
	}
	allowedSet := map[string]bool{"--format": true}
	for _, name := range allowed {
		allowedSet[name] = true
	}
	var flags verifyFlags
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		name, value, hasValue, ok := splitVerifyFlag(arg)
		if !ok {
			return "", flags, fmt.Errorf("unexpected extra args for %q", "verify "+command)
		}
		if !allowedSet[name] {
			return "", flags, fmt.Errorf("unexpected extra args for %q", "verify "+command)
		}
		switch name {
		case "--url":
			if !hasValue {
				if i+1 >= len(rest) {
					return "", flags, fmt.Errorf("--url requires a value")
				}
				i++
				value = rest[i]
			}
			flags.URL = value
		case "--feature":
			if !hasValue {
				if i+1 >= len(rest) {
					return "", flags, fmt.Errorf("--feature requires a value")
				}
				i++
				value = rest[i]
			}
			flags.Feature = value
		case "--dry-run":
			flags.DryRun = parseOptionalBoolFlag(value, hasValue)
		case "--run":
			flags.Run = parseOptionalBoolFlag(value, hasValue)
		case "--job-id":
			if !hasValue {
				if i+1 >= len(rest) {
					return "", flags, fmt.Errorf("--job-id requires a value")
				}
				i++
				value = rest[i]
			}
			flags.JobID = value
		case "--user-data":
			if !hasValue {
				if i+1 >= len(rest) {
					return "", flags, fmt.Errorf("--user-data requires a value")
				}
				i++
				value = rest[i]
			}
			flags.UserData = value
		}
	}
	return format, flags, nil
}

func splitVerifyFlag(arg string) (name, value string, hasValue, ok bool) {
	if !strings.HasPrefix(arg, "--") {
		return "", "", false, false
	}
	name, raw, cut := strings.Cut(arg, "=")
	return name, raw, cut, true
}

func parseOptionalBoolFlag(value string, hasValue bool) bool {
	if !hasValue || value == "" || value == "true" || value == "1" {
		return true
	}
	return false
}

func writeVerifyResult(stdout, stderr io.Writer, format string, value any) error {
	if format == "json" {
		if err := writeJSON(stdout, value); err != nil {
			fmt.Fprintf(stderr, "error: writing json: %v\n", err)
			return err
		}
		return nil
	}
	if err := writeJSON(stdout, value); err != nil {
		fmt.Fprintf(stderr, "error: writing text: %v\n", err)
		return err
	}
	return nil
}

func validateVerifyCommand(args []string) string {
	if len(args) == 0 {
		return `uses non-standard zv command "verify"; expected "verify doctor", "verify features", "verify http", "verify gates", or "verify prove"`
	}
	if isSingleHelp(args) {
		return ""
	}
	switch args[0] {
	case "doctor", "features", "http", "gates", "prove":
		if issue := validateVerifySubcommand(args[0], args[1:]); issue != "" {
			return issue
		}
		return ""
	default:
		return `uses non-standard zv command "verify"; expected "verify doctor", "verify features", "verify http", "verify gates", or "verify prove"`
	}
}

func validateVerifySubcommand(name string, args []string) string {
	if isSingleHelp(args) {
		return ""
	}
	allowed := map[string][]string{
		"doctor":   {"--dry-run", "--user-data"},
		"features": {"--feature"},
		"http":     {"--url"},
		"gates":    {"--dry-run", "--run"},
		"prove":    {"--feature", "--dry-run", "--job-id", "--user-data"},
	}
	_, flags, err := parseVerifyFlags(name, args, allowed[name])
	if err != nil {
		return err.Error()
	}
	if name == "prove" && flags.Feature == "" {
		return `"verify prove" requires --feature`
	}
	return ""
}
