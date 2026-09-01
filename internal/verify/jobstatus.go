package verify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const jobStatusView = "/api/jobs/%s?view=status"

// JobProgress is the orchestrator capture-progress object (overlay percent).
type JobProgress struct {
	Done    int `json:"done"`
	Total   int `json:"total"`
	Percent int `json:"percent"`
}

// JobStatusReport is GET /api/jobs/{id}?view=status. Prove never POSTs.
type JobStatusReport struct {
	OK            bool         `json:"ok"`
	URL           string       `json:"url"`
	HTTPStatus    int          `json:"http_status"`
	Status        string       `json:"status"`
	FailureReason string       `json:"failure_reason,omitempty"`
	Progress      *JobProgress `json:"progress,omitempty"`
	Detail        string       `json:"detail,omitempty"`
}

func jobStatusURL(base, id string) string {
	return strings.TrimRight(base, "/") + fmt.Sprintf(jobStatusView, id)
}

func validJobID(id string) bool {
	id = strings.TrimSpace(id)
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHex(r) {
				return false
			}
		}
	}
	return true
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func inspectJobStatus(probe StudioProbe, base, id string) JobStatusReport {
	report := JobStatusReport{URL: jobStatusURL(base, id)}
	if !validJobID(id) {
		report.Detail = "job id must be a UUID"
		return report
	}
	status, body, err := probe.GetJSON(report.URL)
	if err != nil {
		report.Detail = err.Error()
		return report
	}
	report.HTTPStatus = status
	var decoded struct {
		Status        string       `json:"status"`
		FailureReason string       `json:"failure_reason"`
		Progress      *JobProgress `json:"progress"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		report.Detail = fmt.Sprintf("job status is not JSON: %v", err)
		return report
	}
	report.Status = decoded.Status
	report.FailureReason = decoded.FailureReason
	report.Progress = decoded.Progress
	if status != http.StatusOK || decoded.Status == "" {
		report.Detail = fmt.Sprintf("want 200 {status,...}; got %d %s", status, strings.TrimSpace(string(body)))
		return report
	}
	report.OK = true
	return report
}
