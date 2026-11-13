package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Workspace is a team space. Every member has the same rights; only the
// owner can expel members, transfer the space or delete it.
type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OwnerID   string `json:"owner_id"`
	Role      string `json:"role,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// Member is one person in a workspace.
type Member struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	JoinedAt    int64  `json:"joined_at"`
}

// Invitation is a pending invite to a workspace.
type Invitation struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func cleanWorkspaceName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > 80 {
		return "", fmt.Errorf("%w: the workspace name needs 1 to 80 characters", ErrInvalidInput)
	}
	return name, nil
}

func (s *Service) createWorkspaceTx(ctx context.Context, tx *sql.Tx, ownerID, name string) (Workspace, error) {
	now := s.nowMillis()
	w := Workspace{ID: uuid.NewString(), Name: name, OwnerID: ownerID, Role: RoleOwner, CreatedAt: now}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workspaces (id, name, owner_id, created_at) VALUES (?, ?, ?, ?)`, w.ID, name, ownerID, now); err != nil {
		return Workspace{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memberships (workspace_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`, w.ID, ownerID, RoleOwner, now); err != nil {
		return Workspace{}, err
	}
	return w, nil
}

// CreateWorkspace makes a new workspace owned by the user.
func (s *Service) CreateWorkspace(ctx context.Context, userID, rawName string) (Workspace, error) {
	name, err := cleanWorkspaceName(rawName)
	if err != nil {
		return Workspace{}, err
	}
	var w Workspace
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		w, err = s.createWorkspaceTx(ctx, tx, userID, name)
		return err
	})
	return w, err
}

// roleTx returns the user's role in the workspace, or ErrNotFound.
func (s *Service) roleTx(ctx context.Context, q querier, workspaceID, userID string) (string, error) {
	var role string
	err := q.QueryRowContext(ctx, `SELECT role FROM memberships WHERE workspace_id = ? AND user_id = ?`, workspaceID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

// Role returns the user's role in the workspace, or ErrNotFound for a non-member.
func (s *Service) Role(ctx context.Context, workspaceID, userID string) (string, error) {
	return s.roleTx(ctx, s.db, workspaceID, userID)
}

// ListWorkspaces returns the workspaces the user belongs to, oldest first.
func (s *Service) ListWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT w.id, w.name, w.owner_id, m.role, w.created_at FROM workspaces w JOIN memberships m ON m.workspace_id = w.id
		 WHERE m.user_id = ? ORDER BY w.created_at, w.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Workspace{}
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.OwnerID, &w.Role, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// RenameWorkspace changes the name; any member may.
func (s *Service) RenameWorkspace(ctx context.Context, workspaceID, rawName string) error {
	name, err := cleanWorkspaceName(rawName)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE workspaces SET name = ? WHERE id = ?`, name, workspaceID)
	return err
}

// Members lists the people in a workspace, owner first.
func (s *Service) Members(ctx context.Context, workspaceID string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), u.display_name, m.role, m.joined_at FROM memberships m JOIN users u ON u.id = m.user_id
		 WHERE m.workspace_id = ? ORDER BY CASE m.role WHEN 'owner' THEN 0 ELSE 1 END, m.joined_at`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Email, &m.Username, &m.DisplayName, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Invite emails a link that adds the address to the workspace. Any member may invite.
func (s *Service) Invite(ctx context.Context, workspaceID, inviterID, rawEmail string) (Invitation, error) {
	email := normalizeEmail(rawEmail)
	if email == "" {
		return Invitation{}, fmt.Errorf("%w: invalid email address", ErrInvalidInput)
	}
	var inv Invitation
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var already int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM memberships m JOIN users u ON u.id = m.user_id WHERE m.workspace_id = ? AND u.email = ?`, workspaceID, email).Scan(&already); err != nil {
			return err
		}
		if already > 0 {
			return fmt.Errorf("%w: that person is already a member", ErrConflict)
		}
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM workspaces WHERE id = ?`, workspaceID).Scan(&name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM invitations WHERE workspace_id = ? AND email = ? AND accepted_at IS NULL`, workspaceID, email); err != nil {
			return err
		}
		token, err := randomToken()
		if err != nil {
			return err
		}
		now := s.nowMillis()
		inv = Invitation{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: email, CreatedAt: now, ExpiresAt: s.now().Add(inviteTokenTTL).UnixMilli()}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO invitations (id, workspace_id, email, token_hash, invited_by, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			inv.ID, workspaceID, email, hashToken(token), inviterID, now, inv.ExpiresAt); err != nil {
			return err
		}
		return s.enqueueEmail(ctx, tx, email, "Te han invitado a «"+name+"» en ClipMind",
			"Te han invitado al espacio «"+name+"» en ClipMind.\n\nAcepta la invitación con la cuenta de este correo:\n\n"+s.publicURL+"/invitacion?token="+token+"\n\nCaduca en 7 días.")
	})
	return inv, err
}

// Invitations lists the pending invites of a workspace.
func (s *Service) Invitations(ctx context.Context, workspaceID string) ([]Invitation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, email, created_at, expires_at FROM invitations WHERE workspace_id = ? AND accepted_at IS NULL AND expires_at > ? ORDER BY created_at`,
		workspaceID, s.nowMillis())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.CreatedAt, &inv.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// RevokeInvitation cancels a pending invite of the workspace.
func (s *Service) RevokeInvitation(ctx context.Context, workspaceID, invitationID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM invitations WHERE id = ? AND workspace_id = ? AND accepted_at IS NULL`, invitationID, workspaceID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AcceptInvitation adds the user to the invited workspace. The account's
// email must be the one invited, so a forwarded link is useless.
func (s *Service) AcceptInvitation(ctx context.Context, userID, token string) (Workspace, error) {
	var w Workspace
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var id, workspaceID, email string
		var expires int64
		var accepted sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT id, workspace_id, email, expires_at, accepted_at FROM invitations WHERE token_hash = ?`, hashToken(token)).
			Scan(&id, &workspaceID, &email, &expires, &accepted)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (accepted.Valid || expires <= s.nowMillis())) {
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		var userEmail string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(email, '') FROM users WHERE id = ?`, userID).Scan(&userEmail); err != nil {
			return err
		}
		if !strings.EqualFold(userEmail, email) {
			return fmt.Errorf("%w: this invitation is for another email address", ErrForbidden)
		}
		now := s.nowMillis()
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO memberships (workspace_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`, workspaceID, userID, RoleMember, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE invitations SET accepted_at = ? WHERE id = ?`, now, id); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `SELECT w.id, w.name, w.owner_id, m.role, w.created_at FROM workspaces w JOIN memberships m ON m.workspace_id = w.id WHERE w.id = ? AND m.user_id = ?`,
			workspaceID, userID).Scan(&w.ID, &w.Name, &w.OwnerID, &w.Role, &w.CreatedAt)
	})
	return w, err
}

// RemoveMember lets the owner expel a member, or a member leave. The owner
// cannot leave: they transfer the space or delete it.
func (s *Service) RemoveMember(ctx context.Context, workspaceID, actorID, memberID string) error {
	actorRole, err := s.Role(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if actorID != memberID && actorRole != RoleOwner {
		return fmt.Errorf("%w: only the owner can remove members", ErrForbidden)
	}
	memberRole, err := s.Role(ctx, workspaceID, memberID)
	if err != nil {
		return err
	}
	if memberRole == RoleOwner {
		return fmt.Errorf("%w: the owner cannot leave; transfer the workspace or delete it", ErrForbidden)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM memberships WHERE workspace_id = ? AND user_id = ?`, workspaceID, memberID)
	return err
}

// TransferOwnership makes another member the owner; the old owner stays as a member.
func (s *Service) TransferOwnership(ctx context.Context, workspaceID, actorID, newOwnerID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		role, err := s.roleTx(ctx, tx, workspaceID, actorID)
		if err != nil {
			return err
		}
		if role != RoleOwner {
			return fmt.Errorf("%w: only the owner can transfer the workspace", ErrForbidden)
		}
		if _, err := s.roleTx(ctx, tx, workspaceID, newOwnerID); err != nil {
			return fmt.Errorf("%w: the new owner must be a member", ErrInvalidInput)
		}
		for _, stmt := range []struct {
			sql  string
			args []any
		}{
			{`UPDATE memberships SET role = 'member' WHERE workspace_id = ? AND user_id = ?`, []any{workspaceID, actorID}},
			{`UPDATE memberships SET role = 'owner' WHERE workspace_id = ? AND user_id = ?`, []any{workspaceID, newOwnerID}},
			{`UPDATE workspaces SET owner_id = ? WHERE id = ?`, []any{newOwnerID, workspaceID}},
		} {
			if _, err := tx.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteWorkspace removes the workspace and its memberships. The caller
// deletes the workspace's data folder.
func (s *Service) DeleteWorkspace(ctx context.Context, workspaceID, actorID string) error {
	role, err := s.Role(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role != RoleOwner {
		return fmt.Errorf("%w: only the owner can delete the workspace", ErrForbidden)
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID)
	return err
}
