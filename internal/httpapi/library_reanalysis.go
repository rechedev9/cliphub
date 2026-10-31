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
	version, ok := leadingAnalysisVersion(json.NewDecoder(rc))
	return !ok || version < tacticalplan.AnalysisVersion
}

// leadingAnalysisVersion reads analysis_version without decoding the whole
// document, which runs to megabytes. Document marshals schema_version first
// and analysis_version right after it, so any other key there means version 0.
func leadingAnalysisVersion(dec *json.Decoder) (int, bool) {
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return 0, false
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return 0, false
		}
		switch key {
		case "schema_version":
			var skip string
			if err := dec.Decode(&skip); err != nil {
				return 0, false
			}
		case "analysis_version":
			var version int
			if err := dec.Decode(&version); err != nil {
				return 0, false
			}
			return version, true
		default:
			return 0, true
		}
	}
	return 0, true
}
