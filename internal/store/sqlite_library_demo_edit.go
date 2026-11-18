package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *SQLiteLibraryRepository) UpdateDemo(ctx context.Context, id uuid.UUID, patch library.DemoPatch) (library.Demo, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := r.getDemo(ctx, tx, `d.id = ?`, id.String()); err != nil {
			return err
		}
		if patch.Source != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE library_demos SET source = ?, source_manual = 1 WHERE id = ?`, string(*patch.Source), id.String()); err != nil {
				return fmt.Errorf("update demo source: %w", err)
			}
		}
		if patch.Title != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE library_demos SET title = ? WHERE id = ?`, *patch.Title, id.String()); err != nil {
				return fmt.Errorf("update demo title: %w", err)
			}
		}
		if patch.Tags != nil {
			if _, err := tx.ExecContext(ctx, `DELETE FROM library_demo_tags WHERE demo_id = ?`, id.String()); err != nil {
				return fmt.Errorf("clear demo tags: %w", err)
			}
			if err := insertDemoTags(ctx, tx, id, library.NormalizeTags(*patch.Tags)); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE library_demos SET updated_at = ? WHERE id = ?`, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Demo{}, err
	}
	return r.GetDemo(ctx, id)
}
