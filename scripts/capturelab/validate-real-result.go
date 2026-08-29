package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func main() {
	resultPath := flag.String("result", "", "path to recording-result.json")
	killPlanPath := flag.String("killplan", "", "path to the exact canary kill plan")
	flag.Parse()
	if *resultPath == "" || *killPlanPath == "" {
		fmt.Fprintln(os.Stderr, "--result and --killplan are required")
		os.Exit(2)
	}
	// #nosec G304 -- explicit local canary evidence path.
	content, err := os.ReadFile(*resultPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read result: %v\n", err)
		os.Exit(1)
	}
	var result recording.RecordingResult
	if err := json.Unmarshal(content, &result); err != nil {
		fmt.Fprintf(os.Stderr, "decode result: %v\n", err)
		os.Exit(1)
	}
	// #nosec G304 -- explicit local canary evidence path.
	killPlanContent, err := os.ReadFile(*killPlanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read kill plan: %v\n", err)
		os.Exit(1)
	}
	var plan killplan.Plan
	if err := json.Unmarshal(killPlanContent, &plan); err != nil {
		fmt.Fprintf(os.Stderr, "decode kill plan: %v\n", err)
		os.Exit(1)
	}
	outDir := filepath.Dir(*resultPath)
	expected, err := recording.NewPlanFromKillPlan(plan, result.Plan.DemoPath, outDir, result.Plan.Stream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "derive expected recording plan: %v\n", err)
		os.Exit(1)
	}
	expected.Runtime = result.Plan.Runtime
	if err := recording.ValidateRecordingAttempt(expected, outDir, result); err != nil {
		fmt.Fprintf(os.Stderr, "validate recording attempt: %v\n", err)
		os.Exit(1)
	}
	if err := recording.ValidateUploadResult(result); err != nil {
		fmt.Fprintf(os.Stderr, "validate recording artifacts: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(`{"ok":true,"validator":"recording.ValidateRecordingAttempt+ValidateUploadResult"}`)
}
