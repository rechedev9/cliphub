// Package accounts is the ClipMind catalog: users, sessions, workspaces,
// memberships, invitations and the email outbox. One SQLite file holds it;
// each workspace keeps its own library database elsewhere.
package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Errors a caller maps to HTTP statuses. The messages are safe to show.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotConfirmed  = errors.New("email not confirmed")
	ErrLocked             = errors.New("account temporarily locked")
	ErrTokenInvalid       = errors.New("token invalid or expired")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrInvalidInput       = errors.New("invalid input")
	ErrConflict           = errors.New("conflict")
	ErrUsernameTaken      = errors.New("username taken")
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"

	minPasswordLen  = 8
	maxPasswordLen  = 128
	maxFailedLogins = 5
	lockDuration    = 15 * time.Minute
	confirmTokenTTL = 48 * time.Hour
	resetTokenTTL   = time.Hour
	inviteTokenTTL  = 7 * 24 * time.Hour
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 60 * 24 * time.Hour
)

// Options configure a Service.
type Options struct {
	// SigningKey signs access tokens; at least 32 bytes.
	SigningKey []byte
	// PublicURL prefixes the links in emails.
	PublicURL string
	// PBKDF2Iterations is the password hashing cost; zero means 600000.
	PBKDF2Iterations int
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Service is the account catalog.
type Service struct {
	db         *sql.DB
	key        []byte
	publicURL  string
	iterations int
	now        func() time.Time
}

// Open opens or creates the catalog database at path.
func Open(path string, opts Options) (*Service, error) {
	if len(opts.SigningKey) < 32 {
		return nil, fmt.Errorf("accounts: signing key must be at least 32 bytes")
	}
	db, err := sql.Open("sqlite", path+"?"+strings.Join([]string{
		"_pragma=busy_timeout(5000)",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(1)",
		"_txlock=immediate",
	}, "&"))
	if err != nil {
		return nil, fmt.Errorf("accounts: open: %w", err)
	}
	if strings.Contains(path, ":memory:") || strings.Contains(path, "mode=memory") {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(8)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("accounts: open: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("accounts: schema: %w", err)
	}
	if err := migrateUsers(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("accounts: migrate users: %w", err)
	}
	s := &Service{db: db, key: opts.SigningKey, publicURL: strings.TrimRight(opts.PublicURL, "/"), iterations: opts.PBKDF2Iterations, now: opts.Now}
	if s.iterations <= 0 {
		s.iterations = 600000
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s, nil
}

// Close releases the database.
func (s *Service) Close() error { return s.db.Close() }

func (s *Service) nowMillis() int64 { return s.now().UnixMilli() }

func (s *Service) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// usersColumns is the users table; migrateUsers rebuilds an older one to it.
// An account has an email, a username or both; email_confirmed_at marks it active.
const usersColumns = `(
	id                 TEXT PRIMARY KEY,
	email              TEXT UNIQUE COLLATE NOCASE,
	username           TEXT UNIQUE COLLATE NOCASE,
	password_hash      TEXT NOT NULL,
	display_name       TEXT NOT NULL DEFAULT '',
	email_confirmed_at INTEGER,
	failed_logins      INTEGER NOT NULL DEFAULT 0,
	locked_until       INTEGER NOT NULL DEFAULT 0,
	created_at         INTEGER NOT NULL,
	CHECK (email IS NOT NULL OR username IS NOT NULL)
)`

const schema = `
CREATE TABLE IF NOT EXISTS users ` + usersColumns + `;
CREATE TABLE IF NOT EXISTS email_tokens (
	token_hash TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	kind       TEXT NOT NULL CHECK (kind IN ('confirm','reset')),
	expires_at INTEGER NOT NULL,
	used_at    INTEGER
);
CREATE TABLE IF NOT EXISTS sessions (
	id                TEXT PRIMARY KEY,
	user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	device            TEXT NOT NULL DEFAULT '',
	refresh_hash      TEXT NOT NULL UNIQUE,
	prev_refresh_hash TEXT NOT NULL DEFAULT '',
	workspace_id      TEXT NOT NULL DEFAULT '',
	created_at        INTEGER NOT NULL,
	last_used_at      INTEGER NOT NULL,
	expires_at        INTEGER NOT NULL,
	revoked_at        INTEGER
);
CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id);
CREATE TABLE IF NOT EXISTS workspaces (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	owner_id   TEXT NOT NULL REFERENCES users(id),
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS memberships (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	role         TEXT NOT NULL CHECK (role IN ('owner','member')),
	joined_at    INTEGER NOT NULL,
	PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX IF NOT EXISTS memberships_user ON memberships(user_id);
CREATE TABLE IF NOT EXISTS invitations (
	id           TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	email        TEXT NOT NULL COLLATE NOCASE,
	token_hash   TEXT NOT NULL UNIQUE,
	invited_by   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at   INTEGER NOT NULL,
	expires_at   INTEGER NOT NULL,
	accepted_at  INTEGER
);
CREATE TABLE IF NOT EXISTS api_tokens (
	id           TEXT PRIMARY KEY,
	user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	name         TEXT NOT NULL,
	token_hash   TEXT NOT NULL UNIQUE,
	created_at   INTEGER NOT NULL,
	last_used_at INTEGER NOT NULL DEFAULT 0,
	revoked_at   INTEGER
);
CREATE TABLE IF NOT EXISTS email_outbox (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	to_addr    TEXT NOT NULL,
	subject    TEXT NOT NULL,
	body       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	sent_at    INTEGER,
	attempts   INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT ''
);
`
