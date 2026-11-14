package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User is the public part of an account. Email or Username may be empty, never both.
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CreatedAt   int64  `json:"created_at"`
}

// SignupInput is a new account request.
type SignupInput struct {
	Email       string
	Username    string
	Password    string
	DisplayName string
}

// NewUserInput is an account an operator creates, already active. Email is optional.
type NewUserInput struct {
	Username    string
	Email       string
	Password    string
	DisplayName string
}

// usernamePattern has no "@", so a login with one is an email.
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// normalizeUsername trims and lowercases; it returns "" for a name the pattern refuses.
func normalizeUsername(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if !usernamePattern.MatchString(name) {
		return ""
	}
	return name
}

func checkUsername(raw string) (string, error) {
	name := normalizeUsername(raw)
	if name == "" {
		return "", fmt.Errorf("%w: the username needs 3 to 32 letters, digits, dots, dashes or underscores, starting with a letter or digit", ErrInvalidInput)
	}
	return name, nil
}

func checkDisplayName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if len([]rune(name)) > 80 {
		return "", fmt.Errorf("%w: the name is too long", ErrInvalidInput)
	}
	return name, nil
}

// usernameTaken reports whether an account other than exceptID uses the name.
func usernameTaken(ctx context.Context, tx *sql.Tx, username, exceptID string) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ? AND id != ?`, username, exceptID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrUsernameTaken
	}
	return nil
}

// normalizeEmail trims and lowercases; it returns "" for an address that is not plain.
func normalizeEmail(raw string) string {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 {
		return ""
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || addr.Name != "" {
		return ""
	}
	return email
}

func checkPassword(password string) error {
	n := len([]rune(password))
	if n < minPasswordLen || n > maxPasswordLen {
		return fmt.Errorf("%w: the password needs %d to %d characters", ErrInvalidInput, minPasswordLen, maxPasswordLen)
	}
	return nil
}

// Signup creates an account and its personal workspace and queues the
// confirmation email. An address that already has an account gets an email
// instead of an error, so the form does not reveal who is registered.
func (s *Service) Signup(ctx context.Context, in SignupInput) error {
	email := normalizeEmail(in.Email)
	if email == "" {
		return fmt.Errorf("%w: invalid email address", ErrInvalidInput)
	}
	username, err := checkUsername(in.Username)
	if err != nil {
		return err
	}
	if err := checkPassword(in.Password); err != nil {
		return err
	}
	name, err := checkDisplayName(in.DisplayName)
	if err != nil {
		return err
	}
	hash, err := s.hashPassword(in.Password)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var id string
		var confirmed sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT id, email_confirmed_at FROM users WHERE email = ?`, email).Scan(&id, &confirmed)
		switch {
		case err == nil && confirmed.Valid:
			return s.enqueueEmail(ctx, tx, email, "Ya tienes una cuenta en ClipMind",
				"Alguien intentó crear una cuenta con este correo, pero ya existe una.\n\nInicia sesión en "+s.publicURL+"/iniciar-sesion o restablece tu contraseña en "+s.publicURL+"/recuperar.\n\nSi no fuiste tú, ignora este mensaje.")
		case err == nil:
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		if err := usernameTaken(ctx, tx, username, id); err != nil {
			return err
		}
		if id != "" {
			// The confirmation link asks for the password: the latest sign-up sets it,
			// so an earlier sign-up by someone else cannot keep the account.
			if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, display_name = ?, username = ? WHERE id = ?`, hash, name, username, id); err != nil {
				return err
			}
			return s.sendConfirmation(ctx, tx, id, email)
		}
		id = uuid.NewString()
		now := s.nowMillis()
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, email, username, password_hash, display_name, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, email, username, hash, name, now); err != nil {
			return err
		}
		if _, err := s.createWorkspaceTx(ctx, tx, id, "Mi espacio"); err != nil {
			return err
		}
		return s.sendConfirmation(ctx, tx, id, email)
	})
}

// CreateUser adds an active account and its personal workspace without a
// confirmation email; clipmind-server's "user add" command calls it.
func (s *Service) CreateUser(ctx context.Context, in NewUserInput) (User, error) {
	username, err := checkUsername(in.Username)
	if err != nil {
		return User{}, err
	}
	var email sql.NullString
	if strings.TrimSpace(in.Email) != "" {
		if email.String = normalizeEmail(in.Email); email.String == "" {
			return User{}, fmt.Errorf("%w: invalid email address", ErrInvalidInput)
		}
		email.Valid = true
	}
	if err := checkPassword(in.Password); err != nil {
		return User{}, err
	}
	name, err := checkDisplayName(in.DisplayName)
	if err != nil {
		return User{}, err
	}
	hash, err := s.hashPassword(in.Password)
	if err != nil {
		return User{}, err
	}
	u := User{ID: uuid.NewString(), Email: email.String, Username: username, DisplayName: name, CreatedAt: s.nowMillis()}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := usernameTaken(ctx, tx, username, ""); err != nil {
			return err
		}
		if email.Valid {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email.String).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return fmt.Errorf("%w: that email already has an account", ErrConflict)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO users (id, email, username, password_hash, display_name, email_confirmed_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			u.ID, email, username, hash, name, u.CreatedAt, u.CreatedAt); err != nil {
			return err
		}
		_, err := s.createWorkspaceTx(ctx, tx, u.ID, "Mi espacio")
		return err
	})
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Service) sendConfirmation(ctx context.Context, tx *sql.Tx, userID, email string) error {
	token, err := s.newEmailToken(ctx, tx, userID, "confirm", confirmTokenTTL)
	if err != nil {
		return err
	}
	return s.enqueueEmail(ctx, tx, email, "Confirma tu correo en ClipMind",
		"Confirma tu correo para activar la cuenta:\n\n"+s.publicURL+"/confirmar?token="+token+"\n\nEl enlace caduca en 48 horas. Si no creaste la cuenta, ignora este mensaje.")
}

func (s *Service) newEmailToken(ctx context.Context, tx *sql.Tx, userID, kind string, ttl time.Duration) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM email_tokens WHERE user_id = ? AND kind = ? AND used_at IS NULL`, userID, kind); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_tokens (token_hash, user_id, kind, expires_at) VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, kind, s.now().Add(ttl).UnixMilli()); err != nil {
		return "", err
	}
	return token, nil
}

// useEmailToken consumes a token of the given kind and returns its user.
func (s *Service) useEmailToken(ctx context.Context, tx *sql.Tx, token, kind string) (string, error) {
	var userID string
	var expires int64
	var used sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT user_id, expires_at, used_at FROM email_tokens WHERE token_hash = ? AND kind = ?`,
		hashToken(token), kind).Scan(&userID, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (used.Valid || expires <= s.nowMillis())) {
		return "", ErrTokenInvalid
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_tokens SET used_at = ? WHERE token_hash = ?`, s.nowMillis(), hashToken(token)); err != nil {
		return "", err
	}
	return userID, nil
}

// ConfirmEmail activates the account the token belongs to.
func (s *Service) ConfirmEmail(ctx context.Context, token, password string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		userID, err := s.useEmailToken(ctx, tx, token, "confirm")
		if err != nil {
			return err
		}
		// Only the person who chose the password confirms; a failure rolls back and keeps the link usable.
		var hash string
		if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
			return err
		}
		if !verifyPassword(password, hash) {
			return ErrInvalidCredentials
		}
		_, err = tx.ExecContext(ctx, `UPDATE users SET email_confirmed_at = COALESCE(email_confirmed_at, ?) WHERE id = ?`, s.nowMillis(), userID)
		return err
	})
}

// authenticate checks the credentials and the lockout and returns the user.
// login is an email when it has an "@" and a username otherwise.
func (s *Service) authenticate(ctx context.Context, login, password string) (User, error) {
	column, key := "username", normalizeUsername(login)
	if strings.Contains(login, "@") {
		column, key = "email", normalizeEmail(login)
	}
	var u User
	var hash string
	var confirmed sql.NullInt64
	var failed int
	var lockedUntil int64
	err := sql.ErrNoRows
	if key != "" {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, COALESCE(email, ''), COALESCE(username, ''), display_name, created_at, password_hash, email_confirmed_at, failed_logins, locked_until
			FROM users WHERE `+column+` = ?`, key).
			Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.CreatedAt, &hash, &confirmed, &failed, &lockedUntil)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// Hash anyway so a missing account costs the same as a wrong password.
		_, _ = s.hashPassword(password)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	now := s.nowMillis()
	if lockedUntil > now {
		return User{}, ErrLocked
	}
	if !verifyPassword(password, hash) {
		failed++
		lock := int64(0)
		if failed >= maxFailedLogins {
			lock = s.now().Add(lockDuration).UnixMilli()
			failed = 0
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET failed_logins = ?, locked_until = ? WHERE id = ?`, failed, lock, u.ID); err != nil {
			return User{}, err
		}
		return User{}, ErrInvalidCredentials
	}
	if failed != 0 || lockedUntil != 0 {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET failed_logins = 0, locked_until = 0 WHERE id = ?`, u.ID); err != nil {
			return User{}, err
		}
	}
	if !confirmed.Valid {
		return User{}, ErrEmailNotConfirmed
	}
	return u, nil
}

// ForgotPassword queues a reset email when the address has an account. It
// never reports whether it did.
func (s *Service) ForgotPassword(ctx context.Context, rawEmail string) error {
	email := normalizeEmail(rawEmail)
	if email == "" {
		return nil
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var id string
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		token, err := s.newEmailToken(ctx, tx, id, "reset", resetTokenTTL)
		if err != nil {
			return err
		}
		return s.enqueueEmail(ctx, tx, email, "Restablece tu contraseña de ClipMind",
			"Restablece tu contraseña con este enlace:\n\n"+s.publicURL+"/restablecer?token="+token+"\n\nCaduca en una hora. Si no lo pediste, ignora este mensaje.")
	})
}

// ResetPassword sets a new password, confirms the address (the reset email
// proved it) and signs the user out everywhere.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if err := checkPassword(password); err != nil {
		return err
	}
	hash, err := s.hashPassword(password)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		userID, err := s.useEmailToken(ctx, tx, token, "reset")
		if err != nil {
			return err
		}
		now := s.nowMillis()
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, failed_logins = 0, locked_until = 0, email_confirmed_at = COALESCE(email_confirmed_at, ?) WHERE id = ?`,
			hash, now, userID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, now, userID)
		return err
	})
}

// GetUser returns one account.
func (s *Service) GetUser(ctx context.Context, id string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, COALESCE(email, ''), COALESCE(username, ''), display_name, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// UserByLogin finds an account by its username or email, as Login reads it.
func (s *Service) UserByLogin(ctx context.Context, login string) (User, error) {
	column, key := "username", normalizeUsername(login)
	if strings.Contains(login, "@") {
		column, key = "email", normalizeEmail(login)
	}
	if key == "" {
		return User{}, ErrNotFound
	}
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, COALESCE(email, ''), COALESCE(username, ''), display_name, created_at FROM users WHERE `+column+` = ?`, key).
		Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// SetPassword is the operator's reset ("clipmind-server user passwd"): a new
// password, the lockout cleared and every session signed out.
func (s *Service) SetPassword(ctx context.Context, userID, password string) error {
	if err := checkPassword(password); err != nil {
		return err
	}
	hash, err := s.hashPassword(password)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, failed_logins = 0, locked_until = 0 WHERE id = ?`, hash, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, s.nowMillis(), userID)
		return err
	})
}
