package httpapi

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// libraryAnalysisStale reports whether a ready demo's stored analysis is
// missing, unreadable or older than tacticalplan.AnalysisVersion, so a scan
// would name its zones and sites differently.
func (h *Handlers) libraryAnalysisStale(id uuid.UUID) bool {
	rc, err := h.storage.Open(artifacts.LibraryTacticalIndexKey(id))
	if err != nil {
		return storage.IsNotExist(err)
	}
	defer rc.Close()
	var doc struct {
		AnalysisVersion int `json:"analysis_version"`
	}
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return true
	}
	return doc.AnalysisVersion < tacticalplan.AnalysisVersion
}
