package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Email is one queued message.
type Email struct {
	ID      int64
	To      string
	Subject string
	Body    string
}

const maxEmailAttempts = 5

func (s *Service) enqueueEmail(ctx context.Context, tx *sql.Tx, to, subject, body string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO email_outbox (to_addr, subject, body, created_at) VALUES (?, ?, ?, ?)`, to, subject, body, s.nowMillis())
	return err
}

// PendingEmails returns unsent messages that have not used up their attempts, oldest first.
func (s *Service) PendingEmails(ctx context.Context, limit int) ([]Email, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, to_addr, subject, body FROM email_outbox WHERE sent_at IS NULL AND attempts < ? ORDER BY id LIMIT ?`, maxEmailAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Email
	for rows.Next() {
		var e Email
		if err := rows.Scan(&e.ID, &e.To, &e.Subject, &e.Body); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkEmailSent records a delivered message.
func (s *Service) MarkEmailSent(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE email_outbox SET sent_at = ?, attempts = attempts + 1, last_error = '' WHERE id = ?`, s.nowMillis(), id)
	return err
}

// MarkEmailFailed records a failed attempt; after five the message is left alone.
func (s *Service) MarkEmailFailed(ctx context.Context, id int64, cause error) error {
	_, err := s.db.ExecContext(ctx, `UPDATE email_outbox SET attempts = attempts + 1, last_error = ? WHERE id = ?`, cause.Error(), id)
	return err
}

// UserExport is everything the catalog holds about one person.
type UserExport struct {
	User       User        `json:"user"`
	Workspaces []Workspace `json:"workspaces"`
	Sessions   []Session   `json:"sessions"`
}

// ExportUser returns the account data for a download request (GDPR access).
// Workspace libraries are exported by the server from each workspace.
func (s *Service) ExportUser(ctx context.Context, userID string) (UserExport, error) {
	u, err := s.GetUser(ctx, userID)
	if err != nil {
		return UserExport{}, err
	}
	workspaces, err := s.ListWorkspaces(ctx, userID)
	if err != nil {
		return UserExport{}, err
	}
	sessions, err := s.ListSessions(ctx, userID, "")
	if err != nil {
		return UserExport{}, err
	}
	return UserExport{User: u, Workspaces: workspaces, Sessions: sessions}, nil
}

// DeleteUser erases the account after a password check. It refuses while the
// user owns a workspace that has other members, and returns the ids of the
// workspaces it deleted so the caller removes their data folders.
func (s *Service) DeleteUser(ctx context.Context, userID, password string) ([]string, error) {
	var email, hash string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(email, ''), password_hash FROM users WHERE id = ?`, userID).Scan(&email, &hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !verifyPassword(password, hash) {
		return nil, ErrInvalidCredentials
	}
	var deleted []string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT w.id, (SELECT COUNT(*) FROM memberships m WHERE m.workspace_id = w.id) FROM workspaces w WHERE w.owner_id = ?`, userID)
		if err != nil {
			return err
		}
		var owned []string
		for rows.Next() {
			var id string
			var members int
			if err := rows.Scan(&id, &members); err != nil {
				_ = rows.Close()
				return err
			}
			if members > 1 {
				_ = rows.Close()
				return fmt.Errorf("%w: transfer or delete the workspaces you own that have other members first", ErrConflict)
			}
			owned = append(owned, id)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range owned {
			if _, err := tx.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id); err != nil {
				return err
			}
		}
		deleted = owned
		if _, err := tx.ExecContext(ctx, `DELETE FROM invitations WHERE email = ? OR invited_by = ?`, email, userID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}
