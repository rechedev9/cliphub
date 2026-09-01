package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FeatureMapStatus is one catalog row plus whether its markdown exists.
type FeatureMapStatus struct {
	Feature
	MapPresent bool     `json:"map_present"`
	Headings   []string `json:"headings_ok,omitempty"`
	Issues     []string `json:"issues,omitempty"`
	CheapOK    bool     `json:"cheap_ok"`
	UserPath   string   `json:"user_path"`
}

// FeatureMapReport is the dumpable map contract.
type FeatureMapReport struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	IndexPresent  bool               `json:"index_present"`
	Features      []FeatureMapStatus `json:"features"`
	Issues        []string           `json:"issues"`
}

// InspectFeatureMap checks the skill-local markdown map against the catalog.
func InspectFeatureMap(root string) FeatureMapReport {
	report := FeatureMapReport{SchemaVersion: SchemaVersion}
	dir := filepath.Join(root, filepath.FromSlash(FeatureMapRelDir))
	index := filepath.Join(dir, "INDEX.md")
	if st, err := os.Stat(index); err == nil && st.Mode().IsRegular() {
		report.IndexPresent = true
	} else {
		report.Issues = append(report.Issues, "missing feature map index references/features/INDEX.md")
	}
	for _, feature := range Features() {
		status := FeatureMapStatus{
			Feature:  feature,
			UserPath: userPathStatus(feature, false),
		}
		path := filepath.Join(dir, feature.MapFile)
		body, err := os.ReadFile(path)
		if err != nil {
			status.Issues = append(status.Issues, fmt.Sprintf("missing feature map %s", feature.MapFile))
			report.Features = append(report.Features, status)
			report.Issues = append(report.Issues, status.Issues...)
			continue
		}
		status.MapPresent = true
		text := string(body)
		var missing []string
		for _, heading := range RequiredFeatureHeadings {
			if strings.Contains(text, heading) {
				status.Headings = append(status.Headings, heading)
			} else {
				missing = append(missing, heading)
			}
		}
		if len(missing) > 0 {
			status.Issues = append(status.Issues, fmt.Sprintf("%s missing headings: %s", feature.MapFile, strings.Join(missing, ", ")))
		}
		if !strings.Contains(text, feature.Route) {
			status.Issues = append(status.Issues, fmt.Sprintf("%s does not name route %s", feature.MapFile, feature.Route))
		}
		if feature.RequiresHLAECS2 && !strings.Contains(text, "HLAE") && !strings.Contains(text, "CS2") {
			status.Issues = append(status.Issues, fmt.Sprintf("%s must name the HLAE/CS2 gap", feature.MapFile))
		}
		status.CheapOK = status.MapPresent && len(status.Issues) == 0
		status.UserPath = userPathStatus(feature, false)
		report.Features = append(report.Features, status)
		report.Issues = append(report.Issues, status.Issues...)
	}
	report.OK = report.IndexPresent && len(report.Issues) == 0
	return report
}

func userPathStatus(feature Feature, captureOK bool) string {
	if feature.RequiresHLAECS2 || feature.RequiresWindowsStudio {
		if !captureOK {
			return "gap"
		}
		return "unproven"
	}
	return "unproven"
}
