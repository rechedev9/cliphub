// Package clipmind is the ClipMind web server: accounts and workspaces in
// front of one orchestrator-shaped library per workspace. It serves analysis
// only. The demo file never reaches it: the browser parses it, or Studio
// parses it and syncs the result.
package clipmind

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/accounts"
	"github.com/rechedev9/cliphub/internal/httpapi"
)

// Config sets up a Server.
type Config struct {
	// DataDir holds catalog.db, signing.key and one folder per workspace.
	DataDir string
	// PublicURL is the address users open; emails link to it.
	PublicURL string
	// Mailer delivers email; nil means LogMailer.
	Mailer Mailer
	// MaxOpenWorkspaces caps the workspaces kept open; zero means 64.
	MaxOpenWorkspaces int
	// TrustProxy reads the client address from X-Forwarded-For.
	TrustProxy bool
	// PBKDF2Iterations is the password cost; zero means the production value.
	PBKDF2Iterations int
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// CS2Path is a CS2 executable on this machine; maps and icons come from its install.
	CS2Path string
	// CommunityURL is the pro data server: empty for the default, "off" for none.
	CommunityURL string
	// FaceitAPIKey enables linking players to FACEIT; empty disables it.
	FaceitAPIKey string
}

// Server is the ClipMind HTTP server.
type Server struct {
	cfg      Config
	accounts *accounts.Service
	tenants  *tenants
	limits   *limiter
	kick     chan struct{}
	cancel   context.CancelFunc
	handler  http.Handler
}

// New opens the catalog and builds the routes. Close releases them.
func New(cfg Config) (*Server, error) {
	if cfg.DataDir == "" {
		return nil, errors.New("clipmind: data dir is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Mailer == nil {
		cfg.Mailer = LogMailer{}
	}
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("clipmind: data dir: %w", err)
	}
	key, err := loadSigningKey(filepath.Join(cfg.DataDir, "signing.key"))
	if err != nil {
		return nil, err
	}
	svc, err := accounts.Open(filepath.Join(cfg.DataDir, "catalog.db"), accounts.Options{
		SigningKey: key, PublicURL: cfg.PublicURL, PBKDF2Iterations: cfg.PBKDF2Iterations, Now: cfg.Now,
	})
	if err != nil {
		return nil, err
	}
	sh, err := openShared(cfg)
	if err != nil {
		_ = svc.Close()
		return nil, err
	}
	ts, err := newTenants(filepath.Join(cfg.DataDir, "workspaces"), cfg.MaxOpenWorkspaces, sh.opts)
	if err != nil {
		_ = svc.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{cfg: cfg, accounts: svc, tenants: ts, limits: newLimiter(cfg.Now), kick: make(chan struct{}, 1), cancel: cancel}
	go dispatchEmails(ctx, svc, cfg.Mailer, s.kick)
	if sh.community.Reference != nil {
		go sh.community.Reference.Run(ctx)
	}
	s.handler = s.routes()
	return s, nil
}

// Handler is the server's HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

// Close stops the mail dispatcher and closes every database.
func (s *Server) Close() error {
	s.cancel()
	s.tenants.closeAll()
	return s.accounts.Close()
}

// loadSigningKey reads the access-token key, creating it on first start.
func loadSigningKey(path string) ([]byte, error) {
	// #nosec G304 -- path is inside the configured data dir.
	if raw, err := os.ReadFile(path); err == nil {
		key, err := hex.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil || len(key) < 32 {
			return nil, fmt.Errorf("clipmind: %s is not a valid signing key", path)
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("clipmind: write signing key: %w", err)
	}
	return key, nil
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(securityHeaders)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) })

	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", s.limited("signup", 5, 10*time.Minute, s.handleSignup))
		r.Post("/confirm", s.limited("confirm", 10, time.Minute, s.handleConfirm))
		r.Post("/login", s.limited("login", 10, time.Minute, s.handleLogin))
		r.Post("/refresh", s.limited("refresh", 60, time.Minute, s.handleRefresh))
		r.Post("/forgot", s.limited("forgot", 5, 10*time.Minute, s.handleForgot))
		r.Post("/reset", s.limited("reset", 10, time.Minute, s.handleReset))
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			// An assistant's token may ask who it is, and nothing else here.
			r.Get("/me", s.handleMe)
			r.Group(func(r chi.Router) {
				r.Use(requireSession)
				r.Post("/logout", s.handleLogout)
				r.Get("/sessions", s.handleSessions)
				r.Delete("/sessions/{id}", s.handleRevokeSession)
				r.Get("/export", s.handleExport)
				r.Delete("/account", s.handleDeleteAccount)
				r.Get("/tokens", s.handleListTokens)
				r.Post("/tokens", s.handleCreateToken)
				r.Delete("/tokens/{id}", s.handleRevokeToken)
			})
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Group(func(r chi.Router) {
			r.Use(requireSession)
			r.Get("/workspaces", s.handleListWorkspaces)
			r.Post("/workspaces", s.handleCreateWorkspace)
			r.Post("/invitations/accept", s.handleAcceptInvitation)
			r.Route("/workspaces/{id}", func(r chi.Router) {
				r.Use(s.requireMember)
				r.Patch("/", s.handleRenameWorkspace)
				r.Delete("/", s.handleDeleteWorkspace)
				r.Get("/members", s.handleMembers)
				r.Delete("/members/{user}", s.handleRemoveMember)
				r.Post("/transfer", s.handleTransfer)
				r.Get("/invitations", s.handleInvitations)
				r.Post("/invitations", s.handleInvite)
				r.Delete("/invitations/{invitation}", s.handleRevokeInvitation)
			})
		})
		r.Get("/api/sync/artifacts", s.withTenant(permSync, s.handleListArtifacts))
		r.Get("/api/sync/artifacts/{demo}/{name}", s.withTenant(permSync, s.handleGetArtifact))
		r.Put("/api/sync/artifacts/{demo}/{name}", s.withTenant(permSync, s.handlePutArtifact))
		for _, rule := range workspaceRoutes() {
			r.Method(rule.method, rule.pattern, s.workspaceRoute(rule.perm))
		}
	})
	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// limited wraps a public route with a per-address rate limit.
func (s *Server) limited(name string, limit int, span time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limits.allow(name+"|"+clientIP(r, s.cfg.TrustProxy), limit, span) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(span.Seconds())))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
			return
		}
		next(w, r)
	}
}

type ctxKey struct{}

func principalOf(r *http.Request) accounts.Principal {
	p, _ := r.Context().Value(ctxKey{}).(accounts.Principal)
	return p
}

// requireAuth resolves the bearer token to a principal.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			// Studio's MCP adapter sends its credential in the orchestrator's own header.
			token = r.Header.Get("X-ClipHub-Token")
			ok = token != ""
		}
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
			return
		}
		p, err := s.accounts.Authenticate(r.Context(), token)
		if err != nil {
			if errors.Is(err, accounts.ErrTokenInvalid) {
				writeError(w, http.StatusUnauthorized, "unauthorized", "the session expired")
				return
			}
			writeServerError(w, "authenticate", err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
	})
}

// requireSession keeps account management to a person's own sign-in: an
// assistant's API token cannot change members, tokens or the account.
func requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if principalOf(r).APIToken {
			writeError(w, http.StatusForbidden, "forbidden", "an assistant token cannot manage the account")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireMember limits /workspaces/{id}/... to the workspace's members. A
// non-member gets 404, so the id of someone else's workspace reveals nothing.
func (s *Server) requireMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.accounts.Role(r.Context(), chi.URLParam(r, "id"), principalOf(r).UserID); err != nil {
			if errors.Is(err, accounts.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "workspace not found")
				return
			}
			writeServerError(w, "membership", err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// workspaceRoute forwards a request to the workspace's orchestrator handlers
// once the principal's role allows the permission.
func (s *Server) workspaceRoute(need permission) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principalOf(r)
		if p.WorkspaceID == "" {
			writeError(w, http.StatusConflict, "no_workspace", "choose a workspace first")
			return
		}
		if !roleAllows(p.Role, need) {
			writeError(w, http.StatusForbidden, "forbidden", "your role cannot do this")
			return
		}
		tn, err := s.tenants.acquire(p.WorkspaceID)
		if err != nil {
			writeServerError(w, "open workspace", err)
			return
		}
		defer s.tenants.release(tn)

		// A fresh route context: the inner router matches the path on its own.
		ctx := context.WithValue(r.Context(), chi.RouteCtxKey, (*chi.Context)(nil))
		inner := r.Clone(ctx)
		for _, h := range []string{"Authorization", "Cookie", "Origin", "Sec-Fetch-Site", "X-ClipHub-Token", httpapi.SyncDeviceHeader} {
			inner.Header.Del(h)
		}
		inner.Header.Set("X-ClipHub-Token", s.tenants.token)
		inner.Header.Set(httpapi.SyncDeviceHeader, p.SessionID)
		tn.handler.ServeHTTP(w, inner)
	})
}

const maxJSONBody = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "the request body is not valid JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

func writeServerError(w http.ResponseWriter, op string, err error) {
	fmt.Fprintf(os.Stderr, "clipmind: %s: %v\n", op, err)
	writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
}

// writeAccountError maps a catalog error to a status; anything else is a 500.
func writeAccountError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, accounts.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", cleanMessage(err))
	case errors.Is(err, accounts.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is wrong")
	case errors.Is(err, accounts.ErrEmailNotConfirmed):
		writeError(w, http.StatusForbidden, "email_not_confirmed", "confirm your email first")
	case errors.Is(err, accounts.ErrLocked):
		writeError(w, http.StatusTooManyRequests, "locked", "too many failed attempts, try again in a few minutes")
	case errors.Is(err, accounts.ErrTokenInvalid):
		writeError(w, http.StatusBadRequest, "token_invalid", "the link is invalid or has expired")
	case errors.Is(err, accounts.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", cleanMessage(err))
	case errors.Is(err, accounts.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, accounts.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", cleanMessage(err))
	default:
		writeServerError(w, op, err)
	}
}

// cleanMessage drops the sentinel prefix ("invalid input: ") from a wrapped error.
func cleanMessage(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}
