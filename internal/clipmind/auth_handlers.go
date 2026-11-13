package clipmind

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/accounts"
)

func (s *Server) wake() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	in := accounts.SignupInput{Email: req.Email, Username: req.Username, Password: req.Password, DisplayName: req.DisplayName}
	if err := s.accounts.Signup(r.Context(), in); err != nil {
		writeAccountError(w, "signup", err)
		return
	}
	s.wake()
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.ConfirmEmail(r.Context(), req.Token, req.Password); err != nil {
		writeAccountError(w, "confirm", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
		Device   string `json:"device"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	tokens, err := s.accounts.Login(r.Context(), accounts.LoginInput{Login: req.Login, Password: req.Password, Device: req.Device})
	if err != nil {
		writeAccountError(w, "login", err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
		WorkspaceID  string `json:"workspace_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	tokens, err := s.accounts.Refresh(r.Context(), req.RefreshToken, req.WorkspaceID)
	if err != nil {
		writeAccountError(w, "refresh", err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleForgot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.ForgotPassword(r.Context(), req.Email); err != nil {
		writeServerError(w, "forgot password", err)
		return
	}
	s.wake()
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		writeAccountError(w, "reset password", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalOf(r)
	user, err := s.accounts.GetUser(r.Context(), p.UserID)
	if err != nil {
		writeAccountError(w, "me", err)
		return
	}
	workspaces, err := s.accounts.ListWorkspaces(r.Context(), p.UserID)
	if err != nil {
		writeServerError(w, "list workspaces", err)
		return
	}
	var current *accounts.Workspace
	for i := range workspaces {
		if workspaces[i].ID == p.WorkspaceID {
			current = &workspaces[i]
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "workspace": current, "workspaces": workspaces})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.Logout(r.Context(), principalOf(r).SessionID); err != nil {
		writeServerError(w, "logout", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	p := principalOf(r)
	sessions, err := s.accounts.ListSessions(r.Context(), p.UserID, p.SessionID)
	if err != nil {
		writeServerError(w, "list sessions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.RevokeSession(r.Context(), principalOf(r).UserID, chi.URLParam(r, "id")); err != nil {
		writeAccountError(w, "revoke session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleExport serves the account data as a download (GDPR access).
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	export, err := s.accounts.ExportUser(r.Context(), principalOf(r).UserID)
	if err != nil {
		writeAccountError(w, "export", err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="clipmind-account.json"`)
	writeJSON(w, http.StatusOK, export)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	deleted, err := s.accounts.DeleteUser(r.Context(), principalOf(r).UserID, req.Password)
	if err != nil {
		writeAccountError(w, "delete account", err)
		return
	}
	for _, id := range deleted {
		if err := s.tenants.remove(id); err != nil {
			writeServerError(w, "remove workspace data", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.accounts.ListAPITokens(r.Context(), principalOf(r).UserID)
	if err != nil {
		writeServerError(w, "list tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

// handleCreateToken returns the secret once; only its hash is kept.
func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		WorkspaceID string `json:"workspace_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	p := principalOf(r)
	if req.WorkspaceID == "" {
		req.WorkspaceID = p.WorkspaceID
	}
	secret, token, err := s.accounts.CreateAPIToken(r.Context(), p.UserID, req.WorkspaceID, req.Name)
	if err != nil {
		writeAccountError(w, "create token", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"secret": secret, "token": token})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.RevokeAPIToken(r.Context(), principalOf(r).UserID, chi.URLParam(r, "id")); err != nil {
		writeAccountError(w, "revoke token", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
