package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	sessionLifetime = 30 * 24 * time.Hour
	pairingLifetime = 10 * time.Minute
	deviceOnlineTTL = 90 * time.Second
)

var (
	errEmailExists    = errors.New("email already registered")
	errNotFound       = errors.New("not found")
	errPairingClaimed = errors.New("pairing already claimed")
	errForbidden      = errors.New("forbidden")
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Device struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Platform string    `json:"platform"`
	Version  string    `json:"version"`
	Online   bool      `json:"online"`
	LastSeen time.Time `json:"lastSeen"`
}

type Pairing struct {
	DeviceID string    `json:"deviceId"`
	Code     string    `json:"code"`
	Expires  time.Time `json:"expiresAt"`
	Claimed  bool      `json:"claimed"`
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open control database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, now: time.Now}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  platform TEXT NOT NULL,
  version TEXT NOT NULL,
  secret_hash TEXT NOT NULL,
  last_seen INTEGER,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS pairings (
  device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
  code_hash TEXT NOT NULL UNIQUE,
  expires_at INTEGER NOT NULL,
  claimed_at INTEGER
);
CREATE INDEX IF NOT EXISTS sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS devices_user_id ON devices(user_id);
CREATE INDEX IF NOT EXISTS pairings_expires_at ON pairings(expires_at);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate control database: %w", err)
	}
	return nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) < 3 || len(email) > 254 {
		return "", errors.New("invalid email")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || strings.ContainsAny(email, "\r\n") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func (s *Store) Register(ctx context.Context, rawEmail, password string) (User, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return User{}, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	user := User{ID: uuid.NewString(), Email: email}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		user.ID, user.Email, passwordHash, s.now().UTC().Unix(),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, errEmailExists
		}
		return User{}, fmt.Errorf("register user: %w", err)
	}
	return user, nil
}

func (s *Store) Authenticate(ctx context.Context, rawEmail, password string) (User, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return User{}, errInvalidPassword
	}
	var user User
	var passwordHash string
	err = s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = ?`, email,
	).Scan(&user.ID, &user.Email, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, errInvalidPassword
		}
		return User{}, fmt.Errorf("authenticate user: %w", err)
	}
	if !verifyPassword(passwordHash, password) {
		return User{}, errInvalidPassword
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID string) (string, time.Time, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.now().UTC()
	expires := now.Add(sessionLifetime)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		hashSecret(token), userID, expires.Unix(), now.Unix(),
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	return token, expires, nil
}

func (s *Store) SessionUser(ctx context.Context, token string) (User, error) {
	if len(token) != 64 {
		return User{}, errNotFound
	}
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT users.id, users.email FROM sessions JOIN users ON users.id = sessions.user_id
         WHERE sessions.token_hash = ? AND sessions.expires_at > ?`,
		hashSecret(token), s.now().UTC().Unix(),
	).Scan(&user.ID, &user.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, errNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("read session: %w", err)
	}
	return user, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashSecret(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) CreatePairing(ctx context.Context, deviceID, name, platform, version, secret string) (Pairing, error) {
	if _, err := uuid.Parse(deviceID); err != nil {
		return Pairing{}, errors.New("invalid device id")
	}
	name = strings.TrimSpace(name)
	platform = strings.TrimSpace(platform)
	version = strings.TrimSpace(version)
	if name == "" || len(name) > 80 || platform == "" || len(platform) > 40 || version == "" || len(version) > 40 {
		return Pairing{}, errors.New("invalid device metadata")
	}
	if len(secret) != 64 {
		return Pairing{}, errors.New("invalid device secret")
	}
	if _, err := hex.DecodeString(secret); err != nil {
		return Pairing{}, errors.New("invalid device secret")
	}
	code, err := randomPairingCode()
	if err != nil {
		return Pairing{}, err
	}
	now := s.now().UTC()
	expires := now.Add(pairingLifetime)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Pairing{}, fmt.Errorf("begin pairing: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var owner sql.NullString
	var storedSecret string
	err = tx.QueryRowContext(ctx, `SELECT user_id, secret_hash FROM devices WHERE id = ?`, deviceID).Scan(&owner, &storedSecret)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx,
			`INSERT INTO devices (id, name, platform, version, secret_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			deviceID, name, platform, version, hashSecret(secret), now.Unix(),
		)
	case err != nil:
		return Pairing{}, fmt.Errorf("read pairing device: %w", err)
	case owner.Valid:
		return Pairing{}, errPairingClaimed
	case storedSecret != hashSecret(secret):
		return Pairing{}, errForbidden
	default:
		_, err = tx.ExecContext(ctx,
			`UPDATE devices SET name = ?, platform = ?, version = ? WHERE id = ?`,
			name, platform, version, deviceID,
		)
	}
	if err != nil {
		return Pairing{}, fmt.Errorf("store pairing device: %w", err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO pairings (device_id, code_hash, expires_at, claimed_at) VALUES (?, ?, ?, NULL)
         ON CONFLICT(device_id) DO UPDATE SET code_hash = excluded.code_hash, expires_at = excluded.expires_at, claimed_at = NULL`,
		deviceID, hashSecret(code), expires.Unix(),
	)
	if err != nil {
		return Pairing{}, fmt.Errorf("store pairing: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Pairing{}, fmt.Errorf("commit pairing: %w", err)
	}
	return Pairing{DeviceID: deviceID, Code: code, Expires: expires}, nil
}

func (s *Store) ClaimPairing(ctx context.Context, userID, code string) (Device, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	if len(normalized) != 10 {
		return Device{}, errNotFound
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, fmt.Errorf("begin pairing claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var device Device
	var claimed sql.NullInt64
	err = tx.QueryRowContext(ctx,
		`SELECT devices.id, devices.name, devices.platform, devices.version, pairings.claimed_at
         FROM pairings JOIN devices ON devices.id = pairings.device_id
         WHERE pairings.code_hash = ? AND pairings.expires_at > ?`,
		hashSecret(normalized), now.Unix(),
	).Scan(&device.ID, &device.Name, &device.Platform, &device.Version, &claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, errNotFound
	}
	if err != nil {
		return Device{}, fmt.Errorf("read pairing claim: %w", err)
	}
	if claimed.Valid {
		return Device{}, errPairingClaimed
	}
	result, err := tx.ExecContext(ctx,
		`UPDATE devices SET user_id = ? WHERE id = ? AND user_id IS NULL`, userID, device.ID,
	)
	if err != nil {
		return Device{}, fmt.Errorf("claim pairing device: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return Device{}, errPairingClaimed
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pairings SET claimed_at = ? WHERE device_id = ?`, now.Unix(), device.ID); err != nil {
		return Device{}, fmt.Errorf("finish pairing claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Device{}, fmt.Errorf("commit pairing claim: %w", err)
	}
	return device, nil
}

func (s *Store) PairingStatus(ctx context.Context, deviceID, secret string) (bool, error) {
	var owner sql.NullString
	var secretHash string
	err := s.db.QueryRowContext(ctx, `SELECT user_id, secret_hash FROM devices WHERE id = ?`, deviceID).Scan(&owner, &secretHash)
	if errors.Is(err, sql.ErrNoRows) || secretHash != hashSecret(secret) {
		return false, errForbidden
	}
	if err != nil {
		return false, fmt.Errorf("read pairing status: %w", err)
	}
	return owner.Valid, nil
}

func (s *Store) Heartbeat(ctx context.Context, deviceID, secret, version string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE devices SET last_seen = ?, version = ? WHERE id = ? AND secret_hash = ? AND user_id IS NOT NULL`,
		s.now().UTC().Unix(), strings.TrimSpace(version), deviceID, hashSecret(secret),
	)
	if err != nil {
		return fmt.Errorf("update device heartbeat: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errForbidden
	}
	return nil
}

func (s *Store) Devices(ctx context.Context, userID string) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, platform, version, COALESCE(last_seen, 0) FROM devices WHERE user_id = ? ORDER BY created_at`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()
	devices := []Device{}
	now := s.now().UTC()
	for rows.Next() {
		var device Device
		var lastSeen int64
		if err := rows.Scan(&device.ID, &device.Name, &device.Platform, &device.Version, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		if lastSeen > 0 {
			device.LastSeen = time.Unix(lastSeen, 0).UTC()
			device.Online = now.Sub(device.LastSeen) <= deviceOnlineTTL
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}
	return devices, nil
}

func (s *Store) DeleteDevice(ctx context.Context, userID, deviceID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM devices WHERE id = ? AND user_id = ?`, deviceID, userID)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errNotFound
	}
	return nil
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func randomPairingCode() (string, error) {
	value := make([]byte, 7)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate pairing code: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value)[:10], nil
}

func hashSecret(secret string) string {
	digest := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(digest[:])
}
