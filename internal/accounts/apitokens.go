package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const apiTokenPrefix = "cmk_"

// APIToken is a long-lived key an assistant (Claude, ChatGPT) uses to read and
// edit one workspace's library. The secret is shown once, at creation.
type APIToken struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id"`
	CreatedAt   int64  `json:"created_at"`
	LastUsedAt  int64  `json:"last_used_at"`
}

// CreateAPIToken makes a token bound to a workspace the user belongs to.
func (s *Service) CreateAPIToken(ctx context.Context, userID, workspaceID, rawName string) (string, APIToken, error) {
	name := strings.TrimSpace(rawName)
	if name == "" || len([]rune(name)) > 80 {
		return "", APIToken{}, fmt.Errorf("%w: the token needs a name of 1 to 80 characters", ErrInvalidInput)
	}
	if _, err := s.Role(ctx, workspaceID, userID); err != nil {
		return "", APIToken{}, err
	}
	secret, err := randomToken()
	if err != nil {
		return "", APIToken{}, err
	}
	token := apiTokenPrefix + secret
	t := APIToken{ID: uuid.NewString(), Name: name, WorkspaceID: workspaceID, CreatedAt: s.nowMillis()}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (id, user_id, workspace_id, name, token_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, userID, workspaceID, name, hashToken(token), t.CreatedAt); err != nil {
		return "", APIToken{}, err
	}
	return token, t, nil
}

// ListAPITokens returns the user's active tokens.
func (s *Service) ListAPITokens(ctx context.Context, userID string) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, workspace_id, created_at, last_used_at FROM api_tokens WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIToken{}
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.WorkspaceID, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeAPIToken ends a token at once.
func (s *Service) RevokeAPIToken(ctx context.Context, userID, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, s.nowMillis(), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) authenticateAPIToken(ctx context.Context, token string) (Principal, error) {
	var id, userID, workspaceID string
	var revoked sql.NullInt64
	var lastUsed int64
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, workspace_id, revoked_at, last_used_at FROM api_tokens WHERE token_hash = ?`, hashToken(token)).
		Scan(&id, &userID, &workspaceID, &revoked, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && revoked.Valid) {
		return Principal{}, ErrTokenInvalid
	}
	if err != nil {
		return Principal{}, err
	}
	now := s.nowMillis()
	if now-lastUsed > 60_000 {
		_, _ = s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now, id)
	}
	p := Principal{UserID: userID, SessionID: "api:" + id, APIToken: true}
	role, err := s.Role(ctx, workspaceID, userID)
	switch {
	case err == nil:
		p.WorkspaceID, p.Role = workspaceID, role
	case !errors.Is(err, ErrNotFound):
		return Principal{}, err
	}
	return p, nil
}
