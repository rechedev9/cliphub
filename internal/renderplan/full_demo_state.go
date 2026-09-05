package renderplan

import "github.com/rechedev9/cliphub/internal/recapplan"

// SameFullDemoRequest compares creative content, excluding approval timestamps
// and plan IDs. A legacy task never owns an editorial request's state.
func SameFullDemoRequest(a, b *recapplan.Snapshot) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Document.PlanHash == b.Document.PlanHash && a.Approval.PlanHash == b.Approval.PlanHash
}
