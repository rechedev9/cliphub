package verify

import (
	"fmt"
	"strings"
)

// FeatureMapStatus describes a compiled catalog entry. Catalog validity is
// metadata validation only; it never proves a live Studio user path.
type FeatureMapStatus struct {
	Feature
	CatalogValid bool     `json:"catalog_valid"`
	Issues       []string `json:"issues,omitempty"`
	CheapOK      bool     `json:"cheap_ok"`
	UserPath     string   `json:"user_path"`
}

type FeatureMapReport struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Source        string             `json:"source"`
	Features      []FeatureMapStatus `json:"features"`
	Issues        []string           `json:"issues"`
}

// InspectFeatureMap validates the catalog compiled into this binary. The root
// argument remains for callers that also inspect repository build scripts.
func InspectFeatureMap(_ string) FeatureMapReport {
	return inspectFeatures(Features())
}

func inspectFeatures(features []Feature) FeatureMapReport {
	report := FeatureMapReport{SchemaVersion: SchemaVersion, Source: FeatureCatalogSource, Issues: []string{}}
	if len(features) == 0 {
		report.Issues = append(report.Issues, "empty feature catalog")
	}
	seen := make(map[string]bool, len(features))
	for _, feature := range features {
		status := FeatureMapStatus{Feature: feature, UserPath: userPathStatus(feature, false)}
		if feature.ID == "" || seen[feature.ID] {
			status.Issues = append(status.Issues, fmt.Sprintf("empty or duplicate feature ID %q", feature.ID))
		}
		seen[feature.ID] = true
		if strings.TrimSpace(feature.Title) == "" || strings.TrimSpace(feature.CheapProof) == "" {
			status.Issues = append(status.Issues, fmt.Sprintf("%s needs a title and proof description", feature.ID))
		}
		if !strings.HasPrefix(feature.Route, "/") || strings.HasPrefix(feature.Route, "//") {
			status.Issues = append(status.Issues, fmt.Sprintf("%s needs a local Studio route", feature.ID))
		}
		if !feature.RequiresHLAECS2 && !feature.RequiresWindowsStudio && !strings.HasPrefix(feature.ProbePath, "/api/") {
			status.Issues = append(status.Issues, fmt.Sprintf("%s needs an API probe path", feature.ID))
		}
		status.CatalogValid = len(status.Issues) == 0
		status.CheapOK = status.CatalogValid
		report.Features = append(report.Features, status)
		report.Issues = append(report.Issues, status.Issues...)
	}
	report.OK = len(report.Issues) == 0
	return report
}

func userPathStatus(feature Feature, captureOK bool) string {
	if (feature.RequiresHLAECS2 || feature.RequiresWindowsStudio) && !captureOK {
		return "gap"
	}
	return "unproven"
}
