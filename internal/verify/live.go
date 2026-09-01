package verify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var liveListKeys = []string{"jobs", "projects", "players", "matches", "assets", "variants", "items"}

// LiveInspect is one read-only GET against a live orchestrator.
// Empty lists and gated 503 {code} are success. This is never a capture Pass
// and never a Studio UI walk.
type LiveInspect struct {
	OK         bool   `json:"ok"`
	URL        string `json:"url"`
	HTTPStatus int    `json:"http_status"`
	Code       string `json:"code,omitempty"`
	ItemCount  *int   `json:"item_count,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

func liveInspectURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func inspectLiveAPI(probe StudioProbe, base, path string) LiveInspect {
	report := LiveInspect{URL: liveInspectURL(base, path)}
	if !strings.HasPrefix(path, "/") {
		report.Detail = "probe path must be an absolute /api/... GET"
		return report
	}
	status, body, err := probe.GetJSON(report.URL)
	if err != nil {
		report.Detail = err.Error()
		return report
	}
	report.HTTPStatus = status
	if status == http.StatusServiceUnavailable {
		var decoded struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(body, &decoded) == nil && decoded.Code != "" {
			report.Code = decoded.Code
			report.OK = true
			report.Detail = "orchestrator answered 503 " + decoded.Code + " (gated). Not a capture Pass."
			return report
		}
	}
	if status != http.StatusOK {
		report.Detail = fmt.Sprintf("want 200 JSON or 503 {code}; got %d", status)
		return report
	}
	count, err := jsonItemCount(body)
	if err != nil {
		report.Detail = err.Error()
		return report
	}
	report.ItemCount = count
	report.OK = true
	return report
}

func jsonItemCount(body []byte) (*int, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("live inspect is not JSON: %w", err)
	}
	switch v := raw.(type) {
	case []any:
		n := len(v)
		return &n, nil
	case map[string]any:
		for _, key := range liveListKeys {
			arr, ok := v[key].([]any)
			if !ok {
				continue
			}
			n := len(arr)
			return &n, nil
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("live inspect JSON must be an object or array")
	}
}
