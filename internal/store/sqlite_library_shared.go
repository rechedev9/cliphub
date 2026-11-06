package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *SQLiteLibraryRepository) AttachDemoFile(ctx context.Context, id uuid.UUID, file library.DemoFile) (library.Demo, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE library_demos SET path = ?, file_name = ?, size_bytes = ?, source = ?, source_manual = 0, updated_at = ?
		WHERE id = ?`, file.Path, file.FileName, file.SizeBytes, string(attachedSource(file.Source)), millis(libraryNow()), id.String())
	if err != nil {
		return library.Demo{}, fmt.Errorf("attach demo file: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return library.Demo{}, err
	} else if n == 0 {
		return library.Demo{}, library.ErrNotFound
	}
	return r.GetDemo(ctx, id)
}
