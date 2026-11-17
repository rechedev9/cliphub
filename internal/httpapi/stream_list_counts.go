package httpapi

import (
	"encoding/json"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// streamPlanCounts counts the plan's moments and the videos they export (a
// joined group is one) from the clips and groups alone; a missing or malformed
// plan counts as zero, which is what an empty draft is.
func streamPlanCounts(plan json.RawMessage) (clips, shorts int) {
	if len(plan) == 0 {
		return 0, 0
	}
	var doc struct {
		Clips  []streamclips.ClipRange `json:"clips"`
		Groups []streamclips.ClipGroup `json:"groups"`
	}
	if err := json.Unmarshal(plan, &doc); err != nil {
		return 0, 0
	}
	outputs := streamclips.EditPlan{Clips: doc.Clips, Groups: doc.Groups}.Outputs()
	return len(doc.Clips), len(outputs)
}
