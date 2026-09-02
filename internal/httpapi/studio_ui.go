package httpapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const studioCapabilityCookie = "cliphub_ui_capability"

type studioResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newStudioResponse() *studioResponse {
	return &studioResponse{header: make(http.Header), status: http.StatusOK}
}

func (w *studioResponse) Header() http.Header         { return w.header }
func (w *studioResponse) WriteHeader(status int)      { w.status = status }
func (w *studioResponse) Write(p []byte) (int, error) { return w.body.Write(p) }

func forwardStudioResponse(w http.ResponseWriter, captured *studioResponse) {
	for key, values := range captured.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(captured.status)
	_, _ = w.Write(captured.body.Bytes())
}

// CreateStudioJob preserves the browser facade's camel-case creation body.
func (h *Handlers) CreateStudioJob(w http.ResponseWriter, r *http.Request) {
	captured := newStudioResponse()
	h.CreateJob(captured, r)
	if captured.status >= http.StatusBadRequest {
		forwardStudioResponse(w, captured)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(captured.body.Bytes(), &body); err != nil {
		internalError(w, "decode created Studio job", err)
		return
	}
	writeJSON(w, captured.status, map[string]string{"jobId": body.ID})
}

type studioJob struct {
	JobID       string    `json:"jobId"`
	Status      string    `json:"status"`
	Failure     string    `json:"failureReason,omitempty"`
	FileName    string    `json:"fileName,omitempty"`
	SeriesID    string    `json:"seriesId,omitempty"`
	TargetSteam string    `json:"targetSteamId,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

func (h *Handlers) studioJobs(w http.ResponseWriter, r *http.Request, series bool) {
	captured := newStudioResponse()
	h.ListJobs(captured, r)
	if captured.status >= http.StatusBadRequest {
		forwardStudioResponse(w, captured)
		return
	}
	var body struct {
		Jobs []struct {
			ID          string    `json:"id"`
			Status      string    `json:"status"`
			Failure     string    `json:"failure_reason"`
			FileName    string    `json:"demo_file_name"`
			SeriesID    string    `json:"series_id"`
			TargetSteam string    `json:"target_steamid"`
			CreatedAt   time.Time `json:"created_at"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(captured.body.Bytes(), &body); err != nil {
		internalError(w, "decode Studio jobs", err)
		return
	}
	jobs := make([]studioJob, 0, len(body.Jobs))
	for _, item := range body.Jobs {
		jobs = append(jobs, studioJob{JobID: item.ID, Status: item.Status, Failure: item.Failure, FileName: item.FileName, SeriesID: item.SeriesID, TargetSteam: item.TargetSteam, CreatedAt: item.CreatedAt})
	}
	if series {
		writeJSON(w, http.StatusOK, map[string]any{"demos": jobs})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

// ListStudioJobs returns only the fields needed by the persistent match index.
func (h *Handlers) ListStudioJobs(w http.ResponseWriter, r *http.Request) {
	h.studioJobs(w, r, false)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; connect-src 'self'; worker-src 'self' blob:")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=()")
		next.ServeHTTP(w, r)
	})
}

// BootstrapStudioSession authorizes one local browser launch without exposing the API token.
func (h *Handlers) BootstrapStudioSession(w http.ResponseWriter, r *http.Request) {
	if !studioBrowserRequest(r) {
		writeError(w, http.StatusForbidden, "local API host rejected")
		return
	}
	h.uiBootstrapMu.Lock()
	defer h.uiBootstrapMu.Unlock()
	if h.uiBootstrap == "" || h.uiCapability == "" {
		http.Redirect(w, r, "/bootstrap?error=unavailable", http.StatusSeeOther)
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		writeError(w, http.StatusBadRequest, "form body required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/bootstrap?error=capability", http.StatusSeeOther)
		return
	}
	supplied := r.Form.Get("capability")
	if subtle.ConstantTimeCompare([]byte(supplied), []byte(h.uiBootstrap)) != 1 {
		http.Redirect(w, r, "/bootstrap?error=capability", http.StatusSeeOther)
		return
	}
	h.uiBootstrap = ""
	http.SetCookie(w, &http.Cookie{Name: studioCapabilityCookie, Value: h.uiCapability, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/upload", http.StatusSeeOther)
}

func studioBrowserRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		return false
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	site := strings.ToLower(r.Header.Get("Sec-Fetch-Site"))
	if site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" && !originMatchesHost(origin, r.Host) {
		return false
	}
	return true
}

func studioCookie(r *http.Request) (string, bool) {
	var value string
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name != studioCapabilityCookie {
			continue
		}
		value = cookie.Value
		count++
	}
	return value, count == 1
}

// GetStudioJobStatus keeps the legacy UI path on the lightweight status view.
func (h *Handlers) GetStudioJobStatus(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	query.Set("view", "status")
	r.URL.RawQuery = query.Encode()
	captured := newStudioResponse()
	h.GetJob(captured, r)
	if captured.status >= http.StatusBadRequest {
		forwardStudioResponse(w, captured)
		return
	}
	var body struct {
		Status   string `json:"status"`
		Failure  string `json:"failure_reason,omitempty"`
		Progress any    `json:"progress,omitempty"`
	}
	if err := json.Unmarshal(captured.body.Bytes(), &body); err != nil {
		internalError(w, "decode Studio job status", err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// ListStudioSeries maps the route segment to the canonical jobs query.
func (h *Handlers) ListStudioSeries(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	query.Set("series_id", chi.URLParam(r, "seriesId"))
	r.URL.RawQuery = query.Encode()
	h.studioJobs(w, r, true)
}

// StartStudioParse accepts the historical camel-case browser request.
func (h *Handlers) StartStudioParse(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		SteamID       string `json:"steamId"`
		TargetSteamID string `json:"target_steamid"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid parse request")
		return
	}
	steamID := body.SteamID
	if steamID == "" {
		steamID = body.TargetSteamID
	}
	if steamID == "" || (body.SteamID != "" && body.TargetSteamID != "" && body.SteamID != body.TargetSteamID) {
		writeError(w, http.StatusBadRequest, "steamId required")
		return
	}
	payload, err := json.Marshal(map[string]string{"target_steamid": steamID})
	if err != nil {
		internalError(w, "encode Studio parse request", err)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(payload))
	r.ContentLength = int64(len(payload))
	h.StartParse(w, r)
}

// ServeStudio serves immutable Vite assets and falls back to the SPA document.
func (h *Handlers) ServeStudio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
		http.NotFound(w, r)
		return
	}
	root := osDirFS(h.uiDir)
	requested := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if requested != "" {
		if info, err := fs.Stat(root, requested); err == nil && !info.IsDir() {
			if strings.HasPrefix(requested, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			if contentType := mime.TypeByExtension(path.Ext(requested)); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			http.ServeFileFS(w, r, root, requested)
			return
		}
		if path.Ext(requested) != "" {
			http.NotFound(w, r)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFileFS(w, r, root, "index.html")
}

func osDirFS(dir string) fs.FS {
	return os.DirFS(dir)
}
