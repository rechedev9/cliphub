package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const libraryTeamDocumentColumns = `SELECT d.id, d.team_id, t.name, d.title, d.file_name, d.content_type, d.size_bytes, d.sha256, d.created_at, d.updated_at
	FROM library_team_documents d JOIN library_teams t ON t.id = d.team_id`

func scanLibraryTeamDocument(row sqlScanner) (library.TeamDocument, error) {
	var doc library.TeamDocument
	var id, teamID string
	var created, updated int64
	if err := row.Scan(&id, &teamID, &doc.TeamName, &doc.Title, &doc.FileName, &doc.ContentType, &doc.SizeBytes, &doc.SHA256, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.TeamDocument{}, library.ErrNotFound
		}
		return library.TeamDocument{}, err
	}
	var err error
	if doc.ID, err = uuid.Parse(id); err != nil {
		return library.TeamDocument{}, err
	}
	if doc.TeamID, err = uuid.Parse(teamID); err != nil {
		return library.TeamDocument{}, err
	}
	doc.CreatedAt, doc.UpdatedAt = fromMillis(created), fromMillis(updated)
	return doc, nil
}

func (r *SQLiteLibraryRepository) CreateTeamDocument(ctx context.Context, d *library.TeamDocument, content []byte) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := libraryNow()
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, d.TeamID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_team_documents
			(id, team_id, title, file_name, content_type, size_bytes, sha256, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.ID.String(), d.TeamID.String(), d.Title, d.FileName, d.ContentType, int64(len(content)), d.SHA256, content, millis(now), millis(now))
		return err
	})
	if err != nil {
		return err
	}
	stored, err := r.GetTeamDocument(ctx, d.ID)
	if err != nil {
		return err
	}
	*d = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetTeamDocument(ctx context.Context, id uuid.UUID) (library.TeamDocument, error) {
	return scanLibraryTeamDocument(r.db.QueryRowContext(ctx, libraryTeamDocumentColumns+` WHERE d.id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) TeamDocumentContent(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var content []byte
	err := r.db.QueryRowContext(ctx, `SELECT content FROM library_team_documents WHERE id = ?`, id.String()).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, library.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read team document: %w", err)
	}
	return content, nil
}

func (r *SQLiteLibraryRepository) ListTeamDocuments(ctx context.Context, filter library.TeamDocumentFilter) ([]library.TeamDocument, error) {
	query := libraryTeamDocumentColumns
	args := []any{}
	if filter.TeamID != nil {
		query += ` WHERE d.team_id = ?`
		args = append(args, filter.TeamID.String())
	}
	query += ` ORDER BY d.updated_at DESC, d.id ASC LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list team documents: %w", err)
	}
	defer rows.Close()
	docs := []library.TeamDocument{}
	for rows.Next() {
		doc, err := scanLibraryTeamDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (r *SQLiteLibraryRepository) DeleteTeamDocument(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_team_documents WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete team document: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
