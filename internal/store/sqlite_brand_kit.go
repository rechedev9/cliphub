package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rechedev9/cliphub/internal/brandkit"
)

type SQLiteBrandKitRepository struct {
	db *sql.DB
}

func NewSQLiteBrandKitRepository(db *sql.DB) *SQLiteBrandKitRepository {
	return &SQLiteBrandKitRepository{db: db}
}

func (r *SQLiteBrandKitRepository) GetBrandKit(ctx context.Context) (brandkit.Kit, error) {
	var (
		kit                 brandkit.Kit
		logoID, logoSHA     string
		bannerID, bannerSHA string
		updated             int64
	)
	err := r.db.QueryRowContext(ctx, `SELECT logo_id, logo_sha256, watermark_corner, watermark_opacity, banner_id, banner_sha256, banner_start_seconds, banner_end_seconds, channel_name, updated_at FROM brand_kit WHERE id = 1`).
		Scan(&logoID, &logoSHA, &kit.WatermarkCorner, &kit.WatermarkOpacity, &bannerID, &bannerSHA, &kit.BannerStartSeconds, &kit.BannerEndSeconds, &kit.ChannelName, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return brandkit.Default(), nil
	}
	if err != nil {
		return brandkit.Kit{}, fmt.Errorf("get brand kit: %w", err)
	}
	kit.Logo = imageRefColumns(logoID, logoSHA)
	kit.Banner = imageRefColumns(bannerID, bannerSHA)
	kit.UpdatedAt = fromMillis(updated)
	return kit, nil
}

func (r *SQLiteBrandKitRepository) SaveBrandKit(ctx context.Context, kit brandkit.Kit) (brandkit.Kit, error) {
	if err := kit.Validate(); err != nil {
		return brandkit.Kit{}, err
	}
	saved := kit.Copy()
	saved.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	logoID, logoSHA := imageRefValues(saved.Logo)
	bannerID, bannerSHA := imageRefValues(saved.Banner)
	if _, err := r.db.ExecContext(ctx, `INSERT INTO brand_kit (id, logo_id, logo_sha256, watermark_corner, watermark_opacity, banner_id, banner_sha256, banner_start_seconds, banner_end_seconds, channel_name, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET logo_id = excluded.logo_id, logo_sha256 = excluded.logo_sha256, watermark_corner = excluded.watermark_corner,
			watermark_opacity = excluded.watermark_opacity, banner_id = excluded.banner_id, banner_sha256 = excluded.banner_sha256,
			banner_start_seconds = excluded.banner_start_seconds, banner_end_seconds = excluded.banner_end_seconds, channel_name = excluded.channel_name, updated_at = excluded.updated_at`,
		logoID, logoSHA, saved.WatermarkCorner, saved.WatermarkOpacity, bannerID, bannerSHA, saved.BannerStartSeconds, saved.BannerEndSeconds, saved.ChannelName, millis(saved.UpdatedAt)); err != nil {
		return brandkit.Kit{}, fmt.Errorf("save brand kit: %w", err)
	}
	return saved, nil
}

func imageRefColumns(id, sha string) *brandkit.ImageRef {
	if id == "" {
		return nil
	}
	return &brandkit.ImageRef{ID: id, SHA256: sha}
}

func imageRefValues(ref *brandkit.ImageRef) (string, string) {
	if ref == nil {
		return "", ""
	}
	return ref.ID, ref.SHA256
}
