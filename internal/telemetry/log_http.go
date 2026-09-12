package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func newLogIngestBudget(salt [32]byte) *ingestBudget {
	b := newIngestBudget(salt)
	b.sourceRequestLimit = 120
	b.globalRequestLimit = 1200
	// Here the event reservation unit is bytes, charged only for new records.
	b.sourceEventLimit = 32 << 20
	b.globalEventLimit = 128 << 20
	return b
}

func (a *API) ingestLogs(w http.ResponseWriter, r *http.Request) {
	reject := func(status int, code string) {
		// Log labels only: never request bodies, client IPs, keys or diagnostics.
		a.logf("telemetry stage=logs class=%s status=%d", code, status)
		writeError(w, status, code)
	}
	if !secureEqual(r.Header.Get(IngestKeyHeader), a.ingestKey) {
		reject(http.StatusUnauthorized, "unauthorized")
		return
	}
	now := a.now().UTC()
	source := a.logBudget.sourceKey(r.RemoteAddr)
	if !a.logBudget.AllowRequest(now, source) {
		w.Header().Set("Retry-After", "60")
		reject(http.StatusTooManyRequests, "rate_limited")
		return
	}
	if strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])) != "application/json" {
		reject(http.StatusUnsupportedMediaType, "content_type_must_be_json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLogRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var batch LogBatch
	if err := decoder.Decode(&batch); err != nil {
		reject(requestDecodeStatus(err), "invalid_request")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		reject(http.StatusBadRequest, "invalid_request")
		return
	}
	records, err := validateLogBatch(batch, now)
	if err != nil {
		reject(http.StatusUnprocessableEntity, "invalid_log")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	inserted, err := a.store.logs.Insert(ctx, records, now, func(bytes int) (func(), bool) { return a.logBudget.ReserveEvents(now, source, bytes) })
	if err != nil {
		switch {
		case errors.Is(err, ErrIngestBudget):
			w.Header().Set("Retry-After", "3600")
			reject(http.StatusTooManyRequests, "log_budget_exhausted")
		case errors.Is(err, ErrLogConflict):
			reject(http.StatusConflict, "log_identity_conflict")
		default:
			a.logf("telemetry stage=logs class=store_failed error=%v", err)
			reject(http.StatusServiceUnavailable, "storage_unavailable")
		}
		return
	}
	ids := make([]string, len(records))
	for i, record := range records {
		ids[i] = record.ID
	}
	a.logf("telemetry stage=logs class=accepted records=%d inserted=%d", len(records), inserted)
	// Explicit IDs acknowledge durable records, including an idempotent retry.
	writeJSON(w, http.StatusAccepted, struct {
		AcceptedIDs []string `json:"accepted_ids"`
		Inserted    int      `json:"inserted"`
	}{ids, inserted})
}

func (a *API) queryLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := boundedInt(q.Get("limit"), 200, 1, 500)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_limit")
		return
	}
	var after int64
	if q.Get("after") != "" {
		after, err = strconv.ParseInt(q.Get("after"), 10, 64)
		if err != nil || after < 0 {
			writeError(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
	}
	since, err := querySince(q.Get("since"), a.now().UTC().Add(-30*24*time.Hour))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_since")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminQueryTimeout)
	defer cancel()
	page, err := a.store.logs.Query(ctx, LogQuery{SupportCode: strings.ToUpper(q.Get("support_code")), JobID: q.Get("job_id"), SessionID: q.Get("session_id"), Level: q.Get("level"), Event: q.Get("event"), After: after, Limit: limit, Since: since})
	if err != nil {
		if errors.Is(err, ErrInvalidLogQuery) {
			writeError(w, http.StatusBadRequest, "invalid_log_query")
		} else {
			a.logf("telemetry stage=logs class=query_failed error=%v", err)
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable")
		}
		return
	}
	writeJSON(w, http.StatusOK, page)
}
