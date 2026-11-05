package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Default error codes by HTTP status. Every error body carries a `code` so a
// client can branch on it instead of grepping Spanish or English text; a
// handler with a more specific class uses writeCodedError. 503 deliberately
// has no default: the Studio proxy reserves `service_unavailable` for "the
// orchestrator is unreachable", so a Go 503 must name what is missing.
var defaultErrorCodes = map[int]string{
	http.StatusBadRequest:            "invalid_request",
	http.StatusUnauthorized:          "unauthorized",
	http.StatusForbidden:             "forbidden",
	http.StatusNotFound:              "not_found",
	http.StatusConflict:              "conflict",
	http.StatusRequestEntityTooLarge: "payload_too_large",
	http.StatusMisdirectedRequest:    "misdirected_request",
	http.StatusTooManyRequests:       "rate_limited",
	http.StatusInternalServerError:   "internal_error",
	http.StatusNotImplemented:        "not_implemented",
	http.StatusServiceUnavailable:    "not_configured",
	http.StatusGatewayTimeout:        "timeout",
}

func writeError(w http.ResponseWriter, status int, msg string) {
	code, ok := defaultErrorCodes[status]
	if !ok {
		code = "error"
	}
	writeCodedError(w, status, code, msg)
}

func writeCodedError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "error": msg})
}

// internalError logs the underlying error at the boundary and returns a generic
// 500 to the client so driver/SQL/storage internals are not exposed.
func internalError(w http.ResponseWriter, op string, err error) {
	log.Printf("httpapi: %s: %v", op, err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
