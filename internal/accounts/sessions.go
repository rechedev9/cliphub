package accounts

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Tokens is what a login or refresh hands back.
type Tokens struct {
	Access      string `json:"access_token"`
	Refresh     string `json:"refresh_token"`
	ExpiresAt   int64  `json:"expires_at"`
	UserID      string `json:"user_id"`
	WorkspaceID string `json:"workspace_id"`
}

// Principal is an authenticated request: who, from which session, in which
// workspace and with what role there. The role is read from the database on
// every request, so removing a member cuts access at once.
type Principal struct {
	UserID      string
	SessionID   string
	WorkspaceID string
	Role        string
	// APIToken is true for an assistant's token: it reaches the workspace's
	// library but cannot manage the account.
	APIToken bool
}

// LoginInput is a password login. Device names the client, such as "Studio en PC-Luis".
type LoginInput struct {
	Email    string
	Password string
	Device   string
}

// Login checks the password and opens a session in the user's first workspace.
func (s *Service) Login(ctx context.Context, in LoginInput) (Tokens, error) {
	u, err := s.authenticate(ctx, in.Email, in.Password)
	if err != nil {
		return Tokens{}, err
	}
	var workspaceID string
	err = s.db.QueryRowContext(ctx,
		`SELECT workspace_id FROM memberships WHERE user_id = ? ORDER BY CASE role WHEN 'owner' THEN 0 ELSE 1 END, joined_at LIMIT 1`, u.ID).Scan(&workspaceID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, err
	}
	device := strings.TrimSpace(in.Device)
	if len(device) > 80 {
		device = device[:80]
	}
	return s.openSession(ctx, u.ID, workspaceID, device)
}

func (s *Service) openSession(ctx context.Context, userID, workspaceID, device string) (Tokens, error) {
	refresh, err := randomToken()
	if err != nil {
		return Tokens{}, err
	}
	id := uuid.NewString()
	now := s.nowMillis()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, device, refresh_hash, workspace_id, created_at, last_used_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, device, hashToken(refresh), workspaceID, now, now, s.now().Add(refreshTokenTTL).UnixMilli()); err != nil {
		return Tokens{}, err
	}
	return s.tokensFor(userID, id, workspaceID, refresh)
}

func (s *Service) tokensFor(userID, sessionID, workspaceID, refresh string) (Tokens, error) {
	exp := s.now().Add(accessTokenTTL).UnixMilli()
	access, err := s.signAccess(accessClaims{UserID: userID, SessionID: sessionID, Workspace: workspaceID, ExpiresAt: exp})
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: access, Refresh: refresh, ExpiresAt: exp, UserID: userID, WorkspaceID: workspaceID}, nil
}

// Refresh rotates the refresh token and issues a new access token, optionally
// switching the session to another workspace the user belongs to. A refresh
// token that was already rotated means it leaked: the session is revoked.
func (s *Service) Refresh(ctx context.Context, refresh, workspaceID string) (Tokens, error) {
	hash := hashToken(refresh)
	var tokens Tokens
	var reused bool
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var id, userID, current string
		var expires int64
		var revoked sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT id, user_id, workspace_id, expires_at, revoked_at FROM sessions WHERE refresh_hash = ?`, hash).
			Scan(&id, &userID, &current, &expires, &revoked)
		if errors.Is(err, sql.ErrNoRows) {
			res, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE prev_refresh_hash = ? AND revoked_at IS NULL`, s.nowMillis(), hash)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				reused = true
				return nil
			}
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		if revoked.Valid || expires <= s.nowMillis() {
			return ErrTokenInvalid
		}
		target := current
		if workspaceID != "" {
			if _, err := s.roleTx(ctx, tx, workspaceID, userID); err != nil {
				return err
			}
			target = workspaceID
		}
		next, err := randomToken()
		if err != nil {
			return err
		}
		now := s.nowMillis()
		if _, err := tx.ExecContext(ctx,
			`UPDATE sessions SET refresh_hash = ?, prev_refresh_hash = ?, workspace_id = ?, last_used_at = ?, expires_at = ? WHERE id = ?`,
			hashToken(next), hash, target, now, s.now().Add(refreshTokenTTL).UnixMilli(), id); err != nil {
			return err
		}
		tokens, err = s.tokensFor(userID, id, target, next)
		return err
	})
	if err != nil {
		return Tokens{}, err
	}
	if reused {
		return Tokens{}, ErrTokenInvalid
	}
	return tokens, nil
}

// Logout revokes one session.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, s.nowMillis(), sessionID)
	return err
}

// Authenticate checks an access token against its session and membership.
func (s *Service) Authenticate(ctx context.Context, access string) (Principal, error) {
	if strings.HasPrefix(access, apiTokenPrefix) {
		return s.authenticateAPIToken(ctx, access)
	}
	claims, err := s.verifyAccess(access)
	if err != nil {
		return Principal{}, err
	}
	var revoked sql.NullInt64
	var expires int64
	err = s.db.QueryRowContext(ctx, `SELECT revoked_at, expires_at FROM sessions WHERE id = ? AND user_id = ?`, claims.SessionID, claims.UserID).
		Scan(&revoked, &expires)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (revoked.Valid || expires <= s.nowMillis())) {
		return Principal{}, ErrTokenInvalid
	}
	if err != nil {
		return Principal{}, err
	}
	p := Principal{UserID: claims.UserID, SessionID: claims.SessionID, WorkspaceID: claims.Workspace}
	if claims.Workspace != "" {
		role, err := s.roleTx(ctx, s.db, claims.Workspace, claims.UserID)
		if errors.Is(err, ErrNotFound) {
			// Removed from the workspace: the token stays valid but names no workspace.
			p.WorkspaceID = ""
			return p, nil
		}
		if err != nil {
			return Principal{}, err
		}
		p.Role = role
	}
	return p, nil
}

// Session is one signed-in device.
type Session struct {
	ID         string `json:"id"`
	Device     string `json:"device"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt int64  `json:"last_used_at"`
	Current    bool   `json:"current"`
}

// ListSessions returns the user's active sessions, newest use first.
func (s *Service) ListSessions(ctx context.Context, userID, currentID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, device, created_at, last_used_at FROM sessions WHERE user_id = ? AND revoked_at IS NULL AND expires_at > ? ORDER BY last_used_at DESC`,
		userID, s.nowMillis())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var ses Session
		if err := rows.Scan(&ses.ID, &ses.Device, &ses.CreatedAt, &ses.LastUsedAt); err != nil {
			return nil, err
		}
		ses.Current = ses.ID == currentID
		out = append(out, ses)
	}
	return out, rows.Err()
}

// RevokeSession signs one of the user's devices out.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, s.nowMillis(), sessionID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
