package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// migrateUsers rebuilds a users table made before usernames, where email was
// required. SQLite cannot drop NOT NULL in place, so it follows the documented
// copy, drop and rename procedure with foreign keys off for that connection.
func migrateUsers(db *sql.DB) (err error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var has int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'username'`).Scan(&has); err != nil {
		return err
	}
	if has > 0 {
		return nil
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		if _, onErr := conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`); onErr != nil && err == nil {
			err = onErr
		}
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`CREATE TABLE users_new ` + usersColumns,
		`INSERT INTO users_new (id, email, password_hash, display_name, email_confirmed_at, failed_logins, locked_until, created_at)
			SELECT id, email, password_hash, display_name, email_confirmed_at, failed_logins, locked_until, created_at FROM users`,
		`DROP TABLE users`,
		`ALTER TABLE users_new RENAME TO users`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	var table string
	switch err := tx.QueryRowContext(ctx, `SELECT "table" FROM pragma_foreign_key_check LIMIT 1`).Scan(&table); {
	case err == nil:
		return fmt.Errorf("a row in %s lost its user", table)
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	return tx.Commit()
}
