package clipmind

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.accounts.ListWorkspaces(r.Context(), principalOf(r).UserID)
	if err != nil {
		writeServerError(w, "list workspaces", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ws, err := s.accounts.CreateWorkspace(r.Context(), principalOf(r).UserID, req.Name)
	if err != nil {
		writeAccountError(w, "create workspace", err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (s *Server) handleRenameWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.RenameWorkspace(r.Context(), chi.URLParam(r, "id"), req.Name); err != nil {
		writeAccountError(w, "rename workspace", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.accounts.DeleteWorkspace(r.Context(), id, principalOf(r).UserID); err != nil {
		writeAccountError(w, "delete workspace", err)
		return
	}
	if err := s.tenants.remove(id); err != nil {
		writeServerError(w, "remove workspace data", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.accounts.Members(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServerError(w, "list members", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.RemoveMember(r.Context(), chi.URLParam(r, "id"), principalOf(r).UserID, chi.URLParam(r, "user")); err != nil {
		writeAccountError(w, "remove member", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.accounts.TransferOwnership(r.Context(), chi.URLParam(r, "id"), principalOf(r).UserID, req.UserID); err != nil {
		writeAccountError(w, "transfer workspace", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := s.accounts.Invitations(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServerError(w, "list invitations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invitations": invitations})
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	inv, err := s.accounts.Invite(r.Context(), chi.URLParam(r, "id"), principalOf(r).UserID, req.Email)
	if err != nil {
		writeAccountError(w, "invite", err)
		return
	}
	s.wake()
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) handleRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	if err := s.accounts.RevokeInvitation(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "invitation")); err != nil {
		writeAccountError(w, "revoke invitation", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ws, err := s.accounts.AcceptInvitation(r.Context(), principalOf(r).UserID, req.Token)
	if err != nil {
		writeAccountError(w, "accept invitation", err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}
