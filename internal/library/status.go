package library

// Where an entry of the Libreta stands. Archived entries stay readable, but
// lists hide them unless asked.
const (
	StatusActive   = "active"
	StatusPractice = "practice"
	StatusDraft    = "draft"
	StatusReview   = "review"
	StatusArchived = "archived"
)

// Statuses lists every status in the order the UI shows them.
var Statuses = []string{StatusActive, StatusPractice, StatusDraft, StatusReview, StatusArchived}

// ValidStatus reports whether s is one of Statuses.
func ValidStatus(s string) bool {
	for _, status := range Statuses {
		if s == status {
			return true
		}
	}
	return false
}
