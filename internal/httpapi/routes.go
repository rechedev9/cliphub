package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Routes returns a chi router with all orchestrator routes wired.
func Routes(h *Handlers) chi.Router {
	r := chi.NewRouter()
	r.Use(securityHeaders)
	r.Use(h.rateLimiter.middleware)
	r.Use(crossSiteGuard)
	r.Use(h.requireMutationToken)
	r.Use(h.boundHTTPResources)
	r.Get("/healthz", h.Health)
	r.Get("/metrics", h.Metrics)
	if h.uiDir == "" {
		r.Get("/", h.Workbench)
	}
	r.Get("/ui/workspace", h.WorkbenchWorkspace)
	r.Get("/ui/jobs", h.WorkbenchJobs)
	r.Get("/ui/jobs/{id}", h.WorkbenchJob)
	r.Post("/ui/jobs", h.WorkbenchCreateJob)
	r.Post("/ui/jobs/{id}/parse", h.WorkbenchStartParse)
	r.Post("/ui/jobs/{id}/record", h.WorkbenchStartRecording)
	r.Post("/ui/jobs/{id}/render", h.WorkbenchStartRender)
	r.Post("/ui/jobs/{id}/generate", h.WorkbenchStartGenerate)
	r.Get("/api/capabilities", h.GetCapabilities)
	r.Post("/api/session/bootstrap", h.BootstrapStudioSession)
	r.Get("/api/faceit/players", h.LookupFaceitPlayer)
	r.Get("/api/faceit/players/{playerID}/avatar", h.ProxyFaceitAvatar)
	r.Get("/api/faceit/players/{playerID}/matches", h.ListFaceitMatches)
	r.Get("/api/faceit/followed", h.ListFollowedFaceitPlayers)
	r.Post("/api/faceit/followed", h.FollowFaceitPlayer)
	r.Delete("/api/faceit/followed/{playerID}", h.UnfollowFaceitPlayer)
	r.Get("/api/loadouts", h.ListLoadouts)
	r.Get("/api/presets", h.ListPresets)
	r.Get("/api/songs", h.ListSongs)
	r.Get("/api/songs/{id}/audio", h.GetSongAudio)
	r.Get("/api/stream-variants", h.ListStreamVariants)
	r.Put("/api/voice-profiles/{id}", h.PutVoiceProfile)
	r.Get("/api/voice-profiles/{id}", h.GetVoiceProfile)
	r.Get("/api/voice-profiles/{id}/audio", h.GetVoiceProfileAudio)
	r.Delete("/api/voice-profiles/{id}", h.DeleteVoiceProfile)
	r.Post("/api/steam/sharecode", h.ResolveShareCode)
	r.Get("/api/steam/account", h.GetSteamAccount)
	r.Put("/api/steam/account", h.PutSteamAccount)
	r.Delete("/api/steam/account", h.DeleteSteamAccount)
	r.Post("/api/steam/matches/sync", h.SyncSteamMatches)
	r.Post("/api/steam/import", h.ImportShareCode)
	r.Post("/api/jobs", h.CreateJob)
	r.Get("/api/jobs", h.ListJobs)
	r.Get("/api/jobs/{id}", h.GetJob)
	r.Delete("/api/jobs/{id}", h.DeleteJob)
	r.Get("/api/jobs/{id}/plan", h.GetPlan)
	r.Get("/api/jobs/{id}/recap-plan", h.GetRecapPlan)
	r.Get("/api/jobs/{id}/roster", h.GetRoster)
	r.Post("/api/jobs/{id}/parse", h.StartParse)
	r.Post("/api/jobs/{id}/anticheat", h.StartAnticheat)
	r.Get("/api/jobs/{id}/anticheat", h.GetAnticheat)
	r.Get("/api/jobs/{id}/anticheat/dossier/{steamid}", h.GetAnticheatDossier)
	r.Get("/api/jobs/{id}/moments", h.GetMoments)
	r.Post("/api/jobs/{id}/tactical", h.StartTacticalAnalysis)
	r.Get("/api/jobs/{id}/tactical", h.GetTacticalDocument)
	r.Get("/api/jobs/{id}/tactical/status", h.GetTacticalStatus)
	r.Get("/api/jobs/{id}/tactical/rounds/{round}", h.GetTacticalRound)
	r.Get("/api/jobs/{id}/tactical/positions", h.GetTacticalPositions)
	r.Get("/api/jobs/{id}/tactical/aggregate", h.GetTacticalAggregate)
	r.Get("/api/maps/{map}/radar", h.GetMapRadar)
	r.Get("/api/jobs/{id}/final", h.GetFinal)
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	r.Post("/api/jobs/{id}/generate", h.StartGenerate)
	r.Post("/api/jobs/{id}/compose", h.StartComposition)
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
	r.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
	r.Get("/api/jobs/{id}/renders/{variant}/publish", h.GetRenderPublishBoard)
	r.Get("/api/jobs/{id}/renders/{variant}/quality", h.GetRenderQuality)
	r.Get("/api/jobs/{id}/renders/{variant}/pack", h.GetRenderPack)
	r.Get("/api/jobs/{id}/renders/{variant}/edit-document", h.GetRenderEditDocument)
	r.Get("/api/jobs/{id}/renders/{variant}/gallery", h.GetRenderGallery)
	r.Get("/api/jobs/{id}/renders/{variant}/videos/{name}", h.GetRenderVideo)
	r.Delete("/api/jobs/{id}/renders/{variant}/videos/{name}", h.DeleteRenderVideo)
	r.Get("/api/jobs/{id}/renders/{variant}/videos/{name}/publish-assistant", h.GetPublishAssistant)
	r.Get("/api/jobs/{id}/renders/{variant}/covers/{name}", h.GetRenderCover)
	r.Get("/api/jobs/{id}/renders/{variant}/captions/{name}", h.GetRenderCaption)
	r.Get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/gallery", h.GetRenderRevisionGallery)
	r.Get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/videos/{name}", h.GetRenderRevisionVideo)
	r.Get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/covers/{name}", h.GetRenderRevisionCover)
	r.Get("/api/jobs/{id}/renders/{variant}/revisions/{revision}/captions/{name}", h.GetRenderRevisionCaption)
	r.Post("/api/stream-jobs", h.CreateStreamJob)
	r.Get("/api/stream-jobs", h.ListStreamJobs)
	r.Get("/api/stream-jobs/{id}", h.GetStreamJob)
	r.Get("/api/stream-jobs/{id}/source", h.GetStreamSource)
	r.Get("/api/stream-jobs/{id}/edit-plan", h.GetStreamEditPlan)
	r.Put("/api/stream-jobs/{id}/edit-plan", h.PutStreamEditPlan)
	r.Post("/api/stream-jobs/{id}/renders/{variant}", h.StartStreamRender)
	r.Get("/api/stream-jobs/{id}/renders/{variant}", h.GetStreamRender)
	r.Get("/api/stream-jobs/{id}/renders/{variant}/gallery", h.GetStreamGallery)
	r.Get("/api/stream-jobs/{id}/renders/{variant}/videos/{clip_id}", h.GetStreamVideo)
	r.Get("/api/stream-jobs/{id}/renders/{variant}/delivery/{name}", h.GetStreamDeliveryArtifact)
	r.Post("/api/editor/assets", h.CreateEditorAsset)
	r.Get("/api/editor/assets", h.ListEditorAssets)
	r.Post("/api/editor/assets/import", h.ImportEditorAsset)
	r.Get("/api/editor/assets/{id}", h.GetEditorAsset)
	r.Get("/api/editor/assets/{id}/media", h.GetEditorAssetMedia)
	r.Post("/api/editor/projects", h.CreateEditorProject)
	r.Get("/api/editor/projects", h.ListEditorProjects)
	r.Get("/api/editor/projects/{id}", h.GetEditorProject)
	r.Get("/api/editor/projects/{id}/plan", h.GetEditorPlan)
	r.Put("/api/editor/projects/{id}/plan", h.PutEditorPlan)
	r.Post("/api/editor/projects/{id}/preview", h.PreviewEditorPlan)
	r.Post("/api/editor/projects/{id}/render", h.StartEditorRender)
	r.Get("/api/editor/projects/{id}/render", h.GetEditorRender)
	r.Get("/api/editor/projects/{id}/render/video", h.GetEditorRenderVideo)
	r.Get("/api/editor/projects/{id}/render/cover", h.GetEditorRenderCover)
	registerStudioCompatibilityRoutes(r, h)
	if h.uiDir != "" {
		r.NotFound(h.ServeStudio)
	}
	return r
}

func registerStudioCompatibilityRoutes(r chi.Router, h *Handlers) {
	r.Post("/api/demos/scan", h.CreateStudioJob)
	r.Get("/api/demos/jobs", h.ListStudioJobs)
	r.Get("/api/demos/series/{seriesId}", h.ListStudioSeries)
	r.Get("/api/demos/{id}/status", h.GetStudioJobStatus)
	r.Delete("/api/demos/{id}", h.DeleteJob)
	r.Get("/api/demos/{id}/roster", h.GetRoster)
	r.Post("/api/demos/{id}/parse", h.StartStudioParse)
	r.Get("/api/demos/{id}/plan", h.GetPlan)
	r.Get("/api/demos/{id}/recap-plan", h.GetRecapPlan)
	r.Post("/api/demos/{id}/record", h.StartRecording)
	r.Post("/api/demos/{id}/anticheat", h.StartAnticheat)
	r.Get("/api/demos/{id}/anticheat", h.GetAnticheat)
	r.Get("/api/demos/{id}/anticheat/dossier/{steamid}", h.GetAnticheatDossier)
	r.Post("/api/demos/{id}/tactical", h.StartTacticalAnalysis)
	r.Get("/api/demos/{id}/tactical", h.GetTacticalDocument)
	r.Get("/api/demos/{id}/tactical/status", h.GetTacticalStatus)
	r.Get("/api/demos/{id}/tactical/rounds/{round}", h.GetTacticalRound)
	r.Get("/api/demos/{id}/tactical/positions", h.GetTacticalPositions)
	r.Get("/api/demos/{id}/tactical/aggregate", h.GetTacticalAggregate)
	r.Post("/api/demos/{id}/renders/{variant}", h.StartRenderVariant)
	r.Get("/api/demos/{id}/renders/{variant}", h.GetRenderVariant)
	r.Post("/api/demos/{id}/renders/{variant}/review", h.ResolveRenderReview)
	r.Get("/api/demos/{id}/renders/{variant}/videos/{name}", h.GetRenderVideo)
	r.Delete("/api/demos/{id}/renders/{variant}/videos/{name}", h.DeleteRenderVideo)
	r.Get("/api/demos/{id}/renders/{variant}/videos/{name}/publish-assistant", h.GetPublishAssistant)
	r.Get("/api/demos/{id}/renders/{variant}/covers/{name}", h.GetRenderCover)
	r.Post("/api/streams", h.CreateStreamJob)
	r.Get("/api/streams", h.ListStreamJobs)
	r.Get("/api/streams/{id}", h.GetStreamJob)
	r.Get("/api/streams/{id}/source", h.GetStreamSource)
	r.Get("/api/streams/{id}/edit-plan", h.GetStreamEditPlan)
	r.Put("/api/streams/{id}/edit-plan", h.PutStreamEditPlan)
	r.Post("/api/streams/{id}/renders/{variant}", h.StartStreamRender)
	r.Get("/api/streams/{id}/renders/{variant}", h.GetStreamRender)
	r.Get("/api/streams/{id}/renders/{variant}/videos/{clip_id}", h.GetStreamVideo)
	r.Get("/api/streams/{id}/renders/{variant}/delivery/{name}", h.GetStreamDeliveryArtifact)
}

func (h *Handlers) requireMutationToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session/bootstrap" {
			next.ServeHTTP(w, r)
			return
		}
		protectedRead := strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/metrics" ||
			((r.Method == http.MethodGet || r.Method == http.MethodHead) && strings.HasPrefix(r.URL.Path, "/ui/"))
		protected := isMutationMethod(r.Method) || (h.requireReadAuth && protectedRead)
		if !protected {
			next.ServeHTTP(w, r)
			return
		}
		if h.mutationToken == "" {
			if !h.requireReadAuth {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusServiceUnavailable, "session capability is not configured")
			return
		}
		if !h.tokenMatches(r) {
			writeError(w, http.StatusUnauthorized, "session capability required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenMatches reports whether the request carries the configured mutation
// token, using a constant-time comparison to avoid leaking it via timing.
func (h *Handlers) tokenMatches(r *http.Request) bool {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-ClipHub-Token")), []byte(h.mutationToken)) == 1 {
		return true
	}
	cookie, ok := studioCookie(r)
	return ok && studioBrowserRequest(r) && h.uiCapability != "" && subtle.ConstantTimeCompare([]byte(cookie), []byte(h.uiCapability)) == 1
}

func isMutationMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
