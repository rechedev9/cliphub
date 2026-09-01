package verify

import "fmt"

// ProveReport is the fail-closed answer for one mapped feature.
type ProveReport struct {
	SchemaVersion int              `json:"schema_version"`
	OK            bool             `json:"ok"`
	Closed        bool             `json:"closed"`
	Feature       FeatureMapStatus `json:"feature"`
	Gap           *Gap             `json:"gap,omitempty"`
	Detail        string           `json:"detail"`
}

// ProveFeature checks one catalog row. Features that need HLAE/CS2 or a
// Windows Studio walk never return OK on a host that cannot recertify them.
func ProveFeature(root string, host Host, id string) (ProveReport, error) {
	feature, ok := FeatureByID(id)
	if !ok {
		return ProveReport{}, fmt.Errorf("unknown feature %q", id)
	}
	mapped := InspectFeatureMap(root)
	var row FeatureMapStatus
	for _, status := range mapped.Features {
		if status.ID == id {
			row = status
			break
		}
	}
	if row.ID == "" {
		row = FeatureMapStatus{Feature: feature, UserPath: userPathStatus(feature, host.CanRecertifyCapture())}
	}
	report := ProveReport{SchemaVersion: SchemaVersion, Feature: row}
	if feature.RequiresHLAECS2 || feature.RequiresWindowsStudio {
		if !host.CanRecertifyCapture() {
			gap := Gap{
				ID:      ClosedCaptureGapID,
				Class:   "closed",
				Stage:   "capture",
				Message: ClosedCaptureGap,
			}
			report.Gap = &gap
			report.Closed = true
			report.OK = false
			report.Detail = "fail closed: capture/recap cannot be recertified on this machine"
			return report, nil
		}
		gap := Gap{
			ID:      OverlayWalkGapID,
			Class:   "closed",
			Stage:   "studio_walk",
			Message: OverlayWalkGap,
		}
		report.Gap = &gap
		report.Closed = true
		report.OK = false
		report.Detail = "tools may be present; doctor still will not fake a Pass on a live overlay or Full Demo walk"
		return report, nil
	}
	report.OK = row.CheapOK && row.MapPresent
	if !report.OK {
		report.Detail = "cheap proof failed: feature map or headings are incomplete"
		return report, nil
	}
	report.Detail = "cheap proof only: map and headings exist. A Studio user-path walk remains unproven."
	return report, nil
}
