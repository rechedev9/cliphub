package telemetry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	IngestKeyHeader   = "X-ClipHub-Ingest-Key"
	maxRequestBytes   = 64 << 10
	adminQueryTimeout = 5 * time.Second
)

// API exposes two deliberately separate handlers: PublicHandler is safe to put
// behind Tailscale Funnel, while AdminHandler stays tailnet-only and still
// requires a bearer token.
type API struct {
	store      *Store
	ingestKey  string
	adminToken string
	now        func() time.Time
	logf       func(string, ...any)
	budget     *ingestBudget
}

func NewAPI(store *Store, ingestKey, adminToken string) (*API, error) {
	if store == nil {
		return nil, errors.New("telemetry store is required")
	}
	if len(ingestKey) < 24 {
		return nil, errors.New("ingest key must contain at least 24 characters")
	}
	if len(adminToken) < 32 {
		return nil, errors.New("admin token must contain at least 32 characters")
	}
	var sourceSalt [32]byte
	if _, err := rand.Read(sourceSalt[:]); err != nil {
		return nil, fmt.Errorf("create transient source limiter: %w", err)
	}
	return &API{
		store:      store,
		ingestKey:  ingestKey,
		adminToken: adminToken,
		now:        time.Now,
		logf:       log.Printf,
		budget:     newIngestBudget(sourceSalt),
	}, nil
}

func (a *API) PublicHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.publicHealth)
	mux.HandleFunc("POST /v1/ingest", a.ingest)
	return securityHeaders(mux)
}

func (a *API) AdminHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.adminHealth)
	mux.HandleFunc("GET /v1/incidents", a.incidents)
	mux.HandleFunc("GET /v1/stats", a.stats)
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !secureEqual(bearerToken(r.Header.Get("Authorization")), a.adminToken) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="cliphub-telemetry"`)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		mux.ServeHTTP(w, r)
	}))
}

func (a *API) publicHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "cliphub-telemetry", "status": "ok"})
}

func (a *API) adminHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "cliphub-telemetry-admin", "status": "ok"})
}

func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	now := a.now().UTC()
	if !secureEqual(r.Header.Get(IngestKeyHeader), a.ingestKey) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	source := a.budget.sourceKey(r.RemoteAddr)
	if !a.budget.AllowRequest(now, source) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if contentType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "content_type_must_be_json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var batch Batch
	if err := decoder.Decode(&batch); err != nil {
		writeError(w, requestDecodeStatus(err), "invalid_request")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	events, err := ValidateBatch(batch, now)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_event")
		return
	}
	inserted, err := a.store.Insert(r.Context(), events, now, func(inserted int) (func(), bool) {
		return a.budget.ReserveEvents(now, source, inserted)
	})
	if err != nil {
		if errors.Is(err, ErrIngestBudget) {
			w.Header().Set("Retry-After", "3600")
			writeError(w, http.StatusTooManyRequests, "event_budget_exhausted")
			return
		}
		if errors.Is(err, ErrStorageHighWater) {
			a.logf("telemetry stage=ingest class=storage_high_water")
			writeError(w, http.StatusInsufficientStorage, "storage_capacity_reached")
			return
		}
		a.logf("telemetry stage=ingest class=store_failed error=%v", err)
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int{"accepted": len(events), "inserted": inserted})
}

func (a *API) incidents(w http.ResponseWriter, r *http.Request) {
	supportCode := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("support_code")))
	limit, err := boundedInt(r.URL.Query().Get("limit"), 50, 1, 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_limit")
		return
	}
	since, err := querySince(r.URL.Query().Get("since"), a.now().UTC().Add(-30*24*time.Hour))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_since")
		return
	}
	queryContext, cancel := context.WithTimeout(r.Context(), adminQueryTimeout)
	defer cancel()
	events, err := a.store.Incidents(queryContext, IncidentQuery{
		SupportCode: supportCode,
		Since:       since,
		Limit:       limit,
	})
	if err != nil {
		if strings.Contains(err.Error(), "support code") {
			writeError(w, http.StatusBadRequest, "invalid_support_code")
			return
		}
		a.logf("telemetry stage=admin class=incident_query_failed error=%v", err)
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}
	if events == nil {
		events = []Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"support_code": supportCode,
		"events":       events,
	})
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	hours, err := boundedInt(r.URL.Query().Get("hours"), 24, 1, 30*24)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_hours")
		return
	}
	now := a.now().UTC()
	queryContext, cancel := context.WithTimeout(r.Context(), adminQueryTimeout)
	defer cancel()
	summary, err := a.store.Summary(queryContext, now.Add(-time.Duration(hours)*time.Hour), now)
	if err != nil {
		a.logf("telemetry stage=admin class=stats_query_failed error=%v", err)
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}
	if summary.Errors == nil {
		summary.Errors = []ErrorGroup{}
	}
	if summary.Spans == nil {
		summary.Spans = []SpanGroup{}
	}
	writeJSON(w, http.StatusOK, summary)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func bearerToken(value string) string {
	prefix, token, ok := strings.Cut(strings.TrimSpace(value), " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func secureEqual(got, want string) bool {
	if len(got) != len(want) || len(want) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"code": code})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestDecodeStatus(err error) int {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values")
}

func boundedInt(raw string, fallback, minimum, maximum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("value must be between %d and %d", minimum, maximum)
	}
	return value, nil
}

func querySince(raw string, fallback time.Time) (time.Time, error) {
	if raw == "" {
		return fallback.UTC(), nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return value.UTC(), nil
}

type ingestBudget struct {
	mu sync.Mutex

	sourceSalt [32]byte

	requestWindowStart time.Time
	globalRequests     int
	sourceRequests     map[[32]byte]int
	globalRequestLimit int
	sourceRequestLimit int

	eventWindowStart time.Time
	globalEvents     int
	sourceEvents     map[[32]byte]int
	globalEventLimit int
	sourceEventLimit int
}

func newIngestBudget(sourceSalt [32]byte) *ingestBudget {
	return &ingestBudget{
		sourceSalt:         sourceSalt,
		sourceRequests:     make(map[[32]byte]int),
		globalRequestLimit: 600,
		sourceRequestLimit: 30,
		sourceEvents:       make(map[[32]byte]int),
		globalEventLimit:   5_000,
		sourceEventLimit:   500,
	}
}

func (b *ingestBudget) sourceKey(remoteAddress string) [32]byte {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	identity := []byte(host)
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			identity = ipv4
		} else {
			identity = ip.Mask(net.CIDRMask(64, 128))
		}
	}
	hash := sha256.New()
	_, _ = hash.Write(b.sourceSalt[:])
	_, _ = hash.Write(identity)
	var key [32]byte
	copy(key[:], hash.Sum(nil))
	return key
}

func (b *ingestBudget) AllowRequest(now time.Time, source [32]byte) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if windowExpired(b.requestWindowStart, now, time.Minute) {
		b.requestWindowStart = now
		b.globalRequests = 0
		b.sourceRequests = make(map[[32]byte]int)
	}
	if b.globalRequests >= b.globalRequestLimit || b.sourceRequests[source] >= b.sourceRequestLimit {
		return false
	}
	b.globalRequests++
	b.sourceRequests[source]++
	return true
}

func (b *ingestBudget) ReserveEvents(now time.Time, source [32]byte, count int) (func(), bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if count < 1 || count > b.sourceEventLimit {
		return nil, false
	}
	if windowExpired(b.eventWindowStart, now, time.Hour) {
		b.eventWindowStart = now
		b.globalEvents = 0
		b.sourceEvents = make(map[[32]byte]int)
	}
	if b.globalEvents > b.globalEventLimit-count || b.sourceEvents[source] > b.sourceEventLimit-count {
		return nil, false
	}
	reservedWindow := b.eventWindowStart
	b.globalEvents += count
	b.sourceEvents[source] += count
	rollback := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.eventWindowStart != reservedWindow {
			return
		}
		b.globalEvents -= count
		b.sourceEvents[source] -= count
		if b.sourceEvents[source] == 0 {
			delete(b.sourceEvents, source)
		}
	}
	return rollback, true
}

func windowExpired(start, now time.Time, window time.Duration) bool {
	return start.IsZero() || now.Before(start) || now.Sub(start) >= window
}
