package verify

import (
	"fmt"
	"strings"
)

// ProveOptions drives one feature against the live Studio HTTP surface.
type ProveOptions struct {
	Root     string
	Feature  string
	JobID    string
	DryRun   bool
	UserData string
	GOOS     string
	GOARCH   string
	Probe    StudioProbe
	Host     Host
}

// DriveTarget is the machine-readable open URL for one mapped feature.
// Cheap prove never claims the user path passed; this only tells the agent
// where to walk when Studio web_url is known from ports.json.
type DriveTarget struct {
	FeatureID    string `json:"feature_id"`
	Route        string `json:"route"`
	NavLabel     string `json:"nav_label,omitempty"`
	WebURL       string `json:"web_url,omitempty"`
	OpenURL      string `json:"open_url,omitempty"`
	JobStatusURL string `json:"job_status_url,omitempty"`
	ProbeURL     string `json:"probe_url,omitempty"`
}

// ProveReport is the fail-closed answer for one mapped feature.
type ProveReport struct {
	SchemaVersion int              `json:"schema_version"`
	OK            bool             `json:"ok"`
	Closed        bool             `json:"closed"`
	DryRun        bool             `json:"dry_run,omitempty"`
	Executed      bool             `json:"executed"`
	Feature       FeatureMapStatus `json:"feature"`
	Drive         *DriveTarget     `json:"drive,omitempty"`
	Job           *JobStatusReport `json:"job,omitempty"`
	Live          *LiveInspect     `json:"live,omitempty"`
	Studio        *StudioSurface   `json:"studio,omitempty"`
	Gap           *Gap             `json:"gap,omitempty"`
	Planned       []string         `json:"planned,omitempty"`
	Detail        string           `json:"detail"`
}

// ProveFeature checks one catalog row. Features that need HLAE/CS2 never
// return OK on a host that cannot recertify them.
func ProveFeature(root string, host Host, id string) (ProveReport, error) {
	return Prove(ProveOptions{Root: root, Host: host, Feature: id})
}

// Prove inspects one mapped feature. --dry-run never writes jobs.db and
// never enqueues capture (no HTTP). A live prove may GET a read-only API
// or job status. It never POSTs and never claims a capture Pass.
func Prove(opts ProveOptions) (ProveReport, error) {
	feature, ok := FeatureByID(opts.Feature)
	if !ok {
		return ProveReport{}, fmt.Errorf("unknown feature %q", opts.Feature)
	}
	mapped := InspectFeatureMap(opts.Root)
	var row FeatureMapStatus
	for _, status := range mapped.Features {
		if status.ID == opts.Feature {
			row = status
			break
		}
	}

	probe := opts.Probe
	if probe == nil {
		probe = liveProbe{}
	}
	goos := opts.Host.GOOS
	if goos == "" {
		goos = runtimeGOOS(opts.GOOS)
	}
	host := opts.Host
	if host.GOOS == "" {
		host = InspectHostWith(DoctorOptions{
			UserData: opts.UserData,
			GOOS:     opts.GOOS,
			GOARCH:   opts.GOARCH,
			Probe:    probe,
			DryRun:   opts.DryRun,
		})
	}
	if row.ID == "" {
		row = FeatureMapStatus{Feature: feature, UserPath: userPathStatus(feature, host.CanRecertifyCapture())}
	} else {
		row.UserPath = userPathStatus(feature, host.CanRecertifyCapture())
	}

	report := ProveReport{SchemaVersion: SchemaVersion, Feature: row, DryRun: opts.DryRun}
	report.Drive = driveTarget(feature, goos, opts, probe)
	if opts.DryRun {
		report.OK = true
		report.Executed = false
		report.Planned = []string{
			"inspect feature map " + feature.ID,
			"drive " + feature.Route,
			"no jobs.db writes",
			"no capture enqueue",
			"no HTTP",
		}
		if feature.ProbePath != "" {
			report.Planned = append(report.Planned, "GET "+feature.ProbePath)
		}
		if opts.JobID != "" {
			report.Planned = append(report.Planned, "GET /api/jobs/"+opts.JobID+"?view=status")
		}
		report.Detail = "dry-run: no HTTP, no jobs.db mutation, no capture enqueue"
		return report, nil
	}

	if feature.RequiresHLAECS2 || feature.RequiresWindowsStudio {
		if !host.CanRecertifyCapture() {
			gap := closedGap(ClosedCaptureGapID, "capture", ClosedCaptureGap)
			report.Gap = &gap
			report.Closed = true
			report.OK = false
			report.Detail = "fail closed: capture/recap cannot be recertified on this machine"
			return report, nil
		}
		if opts.JobID == "" {
			gap := closedGap(JobIDRequiredGapID, "studio_walk", JobIDRequiredGap)
			report.Gap = &gap
			report.Closed = true
			report.OK = false
			report.Detail = "Studio is up; prove still needs --job-id to inspect live job status. This is not Full Demo Pass."
			return report, nil
		}
		userData := resolveUserData(goos, opts.UserData)
		studio := inspectStudio(goos, userData, probe, false)
		report.Studio = &studio
		job := inspectJobStatus(probe, studio.OrchestratorURL, opts.JobID)
		report.Job = &job
		if !job.OK {
			gap := closedGap(OverlayWalkGapID, "studio_walk", OverlayWalkGap)
			report.Gap = &gap
			report.Closed = true
			report.OK = false
			report.Detail = "live job status GET failed; this is not Full Demo Pass"
			return report, nil
		}
		row.UserPath = "inspected"
		report.Feature = row
		report.Executed = true
		report.OK = true
		report.Detail = "live job inspected (status/progress). Not Full Demo Pass. This CLI does not screenshot Studio."
		return report, nil
	}

	report.OK = row.CheapOK && row.CatalogValid
	if !report.OK {
		report.Detail = "cheap proof failed: feature catalog entry is invalid"
		return report, nil
	}
	userData := resolveUserData(goos, opts.UserData)
	studio := inspectStudio(goos, userData, probe, false)
	report.Studio = &studio
	if !studioIsUp(studio) {
		report.Executed = true
		report.Detail = "cheap proof only: compiled feature catalog is valid. Studio API not live. A user-path walk remains unproven."
		return report, nil
	}
	live := inspectLiveAPI(probe, studio.WebURL, feature.ProbePath)
	report.Live = &live
	report.Executed = true
	if !live.OK {
		report.OK = false
		report.Detail = "Studio is up but live API inspect failed. Not a capture Pass. A user-path walk remains unproven."
		return report, nil
	}
	row.UserPath = "inspected"
	report.Feature = row
	report.Detail = "live API inspected. A Studio user-path walk remains unproven. Not a capture Pass."
	return report, nil
}

func driveTarget(feature Feature, goos string, opts ProveOptions, probe StudioProbe) *DriveTarget {
	drive := DriveTarget{
		FeatureID: feature.ID,
		Route:     feature.Route,
		NavLabel:  feature.NavLabel,
	}
	userData := resolveUserData(goos, opts.UserData)
	studio := inspectStudio(goos, userData, probe, true)
	if studio.WebURL != "" {
		drive.WebURL = studio.WebURL
		drive.OpenURL = strings.TrimRight(studio.WebURL, "/") + feature.Route
	}
	if opts.JobID != "" && studio.OrchestratorURL != "" {
		drive.JobStatusURL = jobStatusURL(studio.OrchestratorURL, opts.JobID)
	}
	if feature.ProbePath != "" && studio.WebURL != "" {
		drive.ProbeURL = liveInspectURL(studio.WebURL, feature.ProbePath)
	}
	return &drive
}
