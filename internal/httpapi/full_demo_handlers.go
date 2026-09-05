package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// PlanFullDemo resolves facts and local media only; it never enters the capture lane.
func (h *Handlers) PlanFullDemo(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if !canGenerateFromStatus(j.Status) || j.KillPlan == nil {
		writeCodedError(w, http.StatusConflict, recapplan.ErrFactsInsufficient, "Parse the selected player before planning Full Demo")
		return
	}
	var req struct {
		Options *recapplan.Options `json:"options"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || req.Options == nil {
		writeCodedError(w, http.StatusBadRequest, "invalid_full_demo_options", "A complete Full Demo options document is required")
		return
	}
	var d recapplan.Document
	err := h.generateIntents.WhileIdle(j.ID, func() error {
		if err := h.requireGenerateRenderIdle(j.ID); err != nil {
			return err
		}
		var err error
		d, err = h.planFullDemo(r.Context(), j, *req.Options)
		if err != nil {
			return err
		}
		return recapplan.SaveDocument(h.storage, j.ID, d)
	})
	if err != nil {
		rejectFullDemo(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (h *Handlers) planFullDemo(ctx context.Context, j job.Job, options recapplan.Options) (recapplan.Document, error) {
	facts, found, err := recapplan.LoadFacts(h.storage, j.ID)
	if err != nil {
		return recapplan.Document{}, err
	}
	if !found {
		return recapplan.Document{}, &recapplan.Error{Code: recapplan.ErrFactsInsufficient, Detail: "This legacy parse has no independent round facts; parse the selected player again"}
	}
	if facts.TargetSteamID64 != j.TargetSteamID || facts.DemoSHA256 != j.KillPlan.Demo.SHA256 {
		return recapplan.Document{}, &recapplan.Error{Code: recapplan.ErrPlanStale, Detail: "Parsed player or demo differs from the round facts"}
	}
	if err := mediaassets.VerifyContent(ctx, h.storage, j.DemoPath, facts.DemoSHA256, 8<<30); err != nil {
		return recapplan.Document{}, &recapplan.Error{Code: recapplan.ErrPlanStale, Detail: "Demo content is missing or changed: " + err.Error()}
	}
	voice := recapplan.VoiceEvidence{Availability: "not_requested", Activity: []recapplan.TickRange{}}
	if options.Audio.Voice.Enabled || options.Editorial.KeepFreezeVoice {
		stored, err := voicecomms.EnsureStored(ctx, h.storage, j.ID, j.DemoPath, facts.DemoSHA256, j.TargetSteamID, h.fullDemoFFmpeg())
		if err != nil {
			return recapplan.Document{}, &recapplan.Error{Code: recapplan.ErrVoiceDecode, Detail: err.Error()}
		}
		voice = recapplan.VoiceFromExtraction(stored)
	}
	assets := []recapplan.AssetEvidence{}
	for _, ref := range options.AssetReferences() {
		if h.editorAssets == nil {
			continue
		}
		assetID, err := uuid.Parse(ref.ID)
		if err != nil {
			return recapplan.Document{}, err
		}
		a, err := h.editorAssets.Get(ctx, assetID)
		if errors.Is(err, mediaassets.ErrNotFound) {
			continue
		}
		if err != nil {
			return recapplan.Document{}, err
		}
		if a.SHA256 != ref.SHA256 {
			return recapplan.Document{}, &recapplan.Error{Code: recapplan.ErrPlanStale, Detail: "Asset hash changed: " + ref.ID}
		}
		if err := mediaassets.VerifyContent(ctx, h.storage, mediaassets.MediaKey(assetID), ref.SHA256, 8<<30); err != nil {
			return recapplan.Document{}, &recapplan.Error{Code: recapplan.ErrAssetMissing, Detail: err.Error()}
		}
		provenance, found, err := mediaassets.LoadProvenance(h.storage, assetID)
		if err != nil {
			return recapplan.Document{}, err
		}
		if !found {
			continue
		}
		evidence, err := recapplan.AssetFromMedia(a, provenance)
		if err != nil {
			return recapplan.Document{}, err
		}
		assets = append(assets, evidence)
	}
	document, err := recapplan.Plan(facts, options, voice, assets, artifacts.FullDemoFactsKey(j.ID))
	if err != nil {
		var typed *recapplan.Error
		if !errors.As(err, &typed) {
			return recapplan.Document{}, errBadRequest(err.Error())
		}
	}
	return document, err
}

func (h *Handlers) GetFullDemoPlan(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	var d recapplan.Document
	var found bool
	var err error
	if raw := chi.URLParam(r, "planID"); raw != "" {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil || id.String() != raw {
			writeError(w, http.StatusBadRequest, "invalid full demo plan id")
			return
		}
		d, found, err = recapplan.LoadDocument(h.storage, j.ID, id)
	} else {
		d, found, err = recapplan.LoadCurrentDocument(h.storage, j.ID)
	}
	if err != nil {
		rejectFullDemo(w, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{"document": nil, "defaults": recapplan.DefaultOptions(), "compatibility": "legacy-until-planned-and-approved"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document": d, "defaults": recapplan.DefaultOptions(), "compatibility": "editorial-v1"})
}

func (h *Handlers) fullDemoFFmpeg() string {
	for _, tool := range h.capabilities.RenderTools {
		if tool.Name == "ZV_FFMPEG_PATH" && tool.Path != "" {
			return tool.Path
		}
	}
	path, _ := exec.LookPath("ffmpeg")
	return path
}

func (h *Handlers) GetFullDemoRenderDocument(w http.ResponseWriter, r *http.Request) {
	kind, ok := renderplan.FullDemoArtifactKind(chi.URLParam(r, "document"))
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "invalid_full_demo_document", "Unknown Full Demo evidence document")
		return
	}
	if chi.URLParam(r, "revision") != "" {
		h.streamRenderVariantRevisionArtifact(w, r, "application/json", kind, "")
		return
	}
	h.streamRenderVariantArtifact(w, r, "application/json", kind, "")
}

func (h *Handlers) approveFullDemo(ctx context.Context, j job.Job, edit *renderplan.EditRequest) error {
	if edit.FullDemo == nil {
		return nil
	}
	canonical, err := recapplan.ResolveApproval(ctx, h.storage, j.ID, j.DemoPath, j.TargetSteamID, h.fullDemoFFmpeg(), *edit.FullDemo)
	if err != nil {
		return err
	}
	edit.FullDemo = &canonical
	return edit.Validate()
}

func rejectFullDemo(w http.ResponseWriter, err error) {
	if isBadRequest(err) {
		writeCodedError(w, http.StatusBadRequest, "invalid_full_demo_options", err.Error())
		return
	}
	var typed *recapplan.Error
	if errors.As(err, &typed) {
		writeCodedError(w, http.StatusConflict, typed.Code, typed.Detail)
		return
	}
	if errors.Is(err, generateintent.ErrActiveRun) || errors.Is(err, errGenerateRenderActive) {
		writeCodedError(w, http.StatusConflict, generateWorkActive, "Wait for the active job before changing its Full Demo plan")
		return
	}
	internalError(w, "full demo plan", err)
}

// Record retries re-enter generate admission with the saved approval. They may
// not manufacture the old recap defaults after a restart or failed capture.
func (h *Handlers) retryApprovedFullDemo(w http.ResponseWriter, r *http.Request, j job.Job, requested *recapplan.Snapshot) bool {
	if requested == nil {
		previous, found, err := h.readGenerateIntent(j.ID)
		if err != nil {
			rejectFullDemo(w, err)
			return true
		}
		if !found || previous.Edit.FullDemo == nil {
			return false
		}
		requested = previous.Edit.FullDemo
	}
	request := struct {
		Preset string                 `json:"preset"`
		Edit   renderplan.EditRequest `json:"edit"`
	}{"gameplay-pov-60", renderplan.FullDemoEditRequest(*requested)}
	b, err := json.Marshal(request)
	if err != nil {
		rejectFullDemo(w, err)
		return true
	}
	copy := r.Clone(r.Context())
	copy.Body = io.NopCloser(bytes.NewReader(b))
	copy.ContentLength = int64(len(b))
	h.StartGenerate(w, copy)
	return true
}
