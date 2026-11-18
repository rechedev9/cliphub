package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// horizontalDraft is Crear horizontal: the 16:9 summary request built from the newest finished version
// of a Short, with the 9:16 options it drops and whether its plays must be recorded again.
type horizontalDraft struct {
	Revision   uuid.UUID                 `json:"revision"`
	CreatedAt  time.Time                 `json:"created_at"`
	Preset     string                    `json:"preset"`
	SegmentIDs []string                  `json:"segment_ids"`
	Edit       renderplan.EditRequest    `json:"edit"`
	Music      *renderplan.MusicSnapshot `json:"music,omitempty"`
	Dropped    []string                  `json:"dropped"`
	Capture    horizontalCapture         `json:"capture"`
}

type horizontalCapture struct {
	Needed            bool     `json:"needed"`
	MissingSegmentIDs []string `json:"missing_segment_ids"`
}

const notAShortCode = "not_a_short"

// GetHorizontalDraft handles GET /api/jobs/{id}/renders/{variant}/horizontal. It saves nothing: the
// producer, or generate_short, creates the summary as a new video and the Short stays as it is.
func (h *Handlers) GetHorizontalDraft(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if preset, ok := editor.PresetByName(variant); !ok || preset.Width > preset.Height {
		writeCodedError(w, http.StatusConflict, notAShortCode, "only a Short has a horizontal version")
		return
	}
	versions, err := h.renderVersions(j.ID, variant)
	if err != nil {
		internalError(w, "list render versions", err)
		return
	}
	version, ok := newestVersion(versions)
	if !ok {
		writeError(w, http.StatusNotFound, "this Short has no finished version")
		return
	}
	short := version.Edit
	if short.Format != renderplan.FormatShort9x16 || short.DeathReel || short.SeparateShorts || len(version.SegmentIDs) == 0 {
		writeCodedError(w, http.StatusConflict, notAShortCode, "a horizontal version is made from a Short of kills")
		return
	}
	capture, ok := captureRequestFor(renderplan.GenerateIntent{Variant: variant, Edit: short, SegmentIDs: version.SegmentIDs})
	if !ok {
		writeCodedError(w, http.StatusConflict, notAShortCode, "only a Short has a horizontal version")
		return
	}
	edit, dropped := renderplan.HorizontalEdit(short, renderplan.ShortCapture{HUDMode: capture.hudMode, PortraitSafeKillfeed: capture.portraitSafeKillfeed})
	missing, err := recording.MissingShortClips(h.storage, recording.ShortClips{JobID: j.ID, SegmentIDs: version.SegmentIDs,
		HUDMode: recording.HUDMode(capture.hudMode), PortraitSafeKillfeed: capture.portraitSafeKillfeed})
	if err != nil {
		internalError(w, "check the Short's clips", err)
		return
	}
	if dropped == nil {
		dropped = []string{}
	}
	writeJSON(w, http.StatusOK, horizontalDraft{Revision: version.Revision, CreatedAt: version.CreatedAt, Preset: editor.PresetGameplayPOV60,
		SegmentIDs: version.SegmentIDs, Edit: edit, Music: version.Music, Dropped: dropped,
		Capture: horizontalCapture{Needed: len(missing) > 0, MissingSegmentIDs: missing}})
}

// newestVersion is the version Mis vídeos shows, else the newest one.
func newestVersion(versions []renderVersion) (renderVersion, bool) {
	for _, v := range versions {
		if v.Current {
			return v, true
		}
	}
	if len(versions) == 0 {
		return renderVersion{}, false
	}
	return versions[0], true
}
