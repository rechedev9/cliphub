package controlplane

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	sessionCookieName = "cliphub_session"
	maxJSONBody       = 16 * 1024
)

type Server struct {
	store        *Store
	publicOrigin *url.URL
	logger       *log.Logger
}

func NewServer(store *Store, publicOrigin string, logger *log.Logger) (*Server, error) {
	origin, err := url.Parse(strings.TrimRight(publicOrigin, "/"))
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.Path != "" {
		return nil, errors.New("public origin must be an absolute HTTP origin without a path")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Server{store: store, publicOrigin: origin, logger: logger}, nil
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(s.securityHeaders)
	router.Use(s.sameOriginMutations)
	router.Get("/healthz", s.health)
	router.Route("/api/account", func(r chi.Router) {
		r.Post("/register", s.register)
		r.Post("/login", s.login)
		r.Post("/logout", s.logout)
		r.With(s.requireUser).Get("/session", s.session)
		r.With(s.requireUser).Get("/devices", s.devices)
		r.With(s.requireUser).Post("/devices/claim", s.claimDevice)
		r.With(s.requireUser).Delete("/devices/{deviceID}", s.deleteDevice)
	})
	router.Route("/api/agent", func(r chi.Router) {
		r.Post("/pairings", s.createPairing)
		r.Post("/pairings/{deviceID}/status", s.pairingStatus)
		r.Post("/heartbeat", s.heartbeat)
	})
	return router
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input credentialsRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.store.Register(r.Context(), input.Email, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, errEmailExists):
			writeJSONError(w, http.StatusConflict, "email_already_registered", "Ya existe una cuenta con ese correo.")
		case strings.Contains(err.Error(), "password"):
			writeJSONError(w, http.StatusBadRequest, "invalid_password", err.Error())
		default:
			writeJSONError(w, http.StatusBadRequest, "invalid_registration", "Revisa el correo y la contraseña.")
		}
		return
	}
	s.createLogin(w, r, user, http.StatusCreated)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input credentialsRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.store.Authenticate(r.Context(), input.Email, input.Password)
	if err != nil {
		if !errors.Is(err, errInvalidPassword) {
			s.logger.Printf("control plane login: %v", err)
		}
		writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "Correo o contraseña incorrectos.")
		return
	}
	s.createLogin(w, r, user, http.StatusOK)
}

func (s *Server) createLogin(w http.ResponseWriter, r *http.Request, user User, status int) {
	token, expires, err := s.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		s.logger.Printf("control plane create session: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "No se pudo iniciar la sesión.")
		return
	}
	// #nosec G124 -- production requires an HTTPS public origin. Conditional
	// Secure=false exists only for loopback integration tests and local smoke tests.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.publicOrigin.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	})
	writeJSON(w, status, map[string]any{"user": user})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookieName)
	if cookie != nil {
		if err := s.store.DeleteSession(r.Context(), cookie.Value); err != nil {
			s.logger.Printf("control plane logout: %v", err)
		}
	}
	// #nosec G124 -- mirrors the session cookie attributes; HTTP is accepted
	// only for loopback tests, while the deployed origin is HTTPS.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.publicOrigin.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

type userContextKey struct{}

func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "authentication_required", "Inicia sesión para continuar.")
			return
		}
		user, err := s.store.SessionUser(r.Context(), cookie.Value)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "authentication_required", "Inicia sesión para continuar.")
			return
		}
		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": userFromContext(r.Context())})
}

func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.Devices(r.Context(), userFromContext(r.Context()).ID)
	if err != nil {
		s.logger.Printf("control plane list devices: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "No se pudieron cargar tus dispositivos.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (s *Server) claimDevice(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	device, err := s.store.ClaimPairing(r.Context(), userFromContext(r.Context()).ID, input.Code)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "pairing_not_found", "El código ha caducado o ya se ha utilizado.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"device": device})
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteDevice(r.Context(), userFromContext(r.Context()).ID, chi.URLParam(r, "deviceID")); err != nil {
		writeJSONError(w, http.StatusNotFound, "device_not_found", "No se encontró el dispositivo.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pairingRequest struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	Secret   string `json:"secret"`
}

func (s *Server) createPairing(w http.ResponseWriter, r *http.Request) {
	var input pairingRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	pairing, err := s.store.CreatePairing(r.Context(), input.DeviceID, input.Name, input.Platform, input.Version, input.Secret)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_pairing", "No se pudo preparar el emparejamiento.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"pairing": pairing})
}

func (s *Server) pairingStatus(w http.ResponseWriter, r *http.Request) {
	secret, ok := bearerToken(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "device_authentication_required", "Falta la identidad del dispositivo.")
		return
	}
	claimed, err := s.store.PairingStatus(r.Context(), chi.URLParam(r, "deviceID"), secret)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid_device", "El dispositivo no está autorizado.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"claimed": claimed})
}

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	secret, ok := bearerToken(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "device_authentication_required", "Falta la identidad del dispositivo.")
		return
	}
	var input struct {
		DeviceID string `json:"deviceId"`
		Version  string `json:"version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.store.Heartbeat(r.Context(), input.DeviceID, secret, input.Version); err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid_device", "El dispositivo no está autorizado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) sameOriginMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != s.publicOrigin.String() {
			writeJSONError(w, http.StatusForbidden, "cross_site_request", "Solicitud bloqueada.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "El contenido de la solicitud no es válido.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "El contenido de la solicitud no es válido.")
		return false
	}
	return true
}

func bearerToken(r *http.Request) (string, bool) {
	value := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(value, "Bearer ")
	return token, ok && len(token) == 64
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "error": message})
}
