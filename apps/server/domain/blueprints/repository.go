package blueprints

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/emergent-company/emergent.memory/pkg/apperror"
	"github.com/emergent-company/emergent.memory/pkg/logger"
)

// Repository handles database operations for blueprints
type Repository struct {
	db  bun.IDB
	log *slog.Logger
}

// NewRepository creates a new blueprints repository
func NewRepository(db bun.IDB, log *slog.Logger) *Repository {
	return &Repository{
		db:  db,
		log: log.With(logger.Scope("blueprints.repo")),
	}
}

// Create inserts a new blueprint row.
// Duplicate (name, version) is enforced atomically by the database via
// ON CONFLICT DO NOTHING (backed by the blueprints_name_version_key unique
// constraint); RowsAffected == 0 means the row already exists. The service's
// ExistsByNameVersion pre-check is only a friendly fast path, not the
// enforcement.
func (r *Repository) Create(ctx context.Context, bp *Blueprint) error {
	result, err := r.db.NewInsert().
		Model(bp).
		On("CONFLICT (name, version) DO NOTHING").
		Returning("id").
		Exec(ctx)
	if err != nil {
		r.log.Error("failed to create blueprint", logger.Error(err))
		return apperror.ErrDatabase.WithInternal(err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperror.ErrConflict.WithMessage("blueprint with name and version already exists")
	}

	return nil
}

// GetByID returns a blueprint by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Blueprint, error) {
	var bp Blueprint
	err := r.db.NewSelect().
		Model(&bp).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrNotFound.WithMessage("blueprint not found")
		}
		r.log.Error("failed to get blueprint", logger.Error(err))
		return nil, apperror.ErrDatabase.WithInternal(err)
	}
	return &bp, nil
}

// List returns blueprints, optionally filtered by name, ordered by name then version.
func (r *Repository) List(ctx context.Context, nameFilter string) ([]Blueprint, error) {
	q := r.db.NewSelect().Model((*Blueprint)(nil))
	if nameFilter != "" {
		q = q.Where("name = ?", nameFilter)
	}

	var blueprints []Blueprint
	err := q.Order("name ASC", "version ASC").Scan(ctx, &blueprints)
	if err != nil {
		r.log.Error("failed to list blueprints", logger.Error(err))
		return nil, apperror.ErrDatabase.WithInternal(err)
	}
	if blueprints == nil {
		return []Blueprint{}, nil
	}
	return blueprints, nil
}

// Update persists the mutable content fields of a draft blueprint in place.
// Status is not touched here; use UpdateStatus for state transitions.
func (r *Repository) Update(ctx context.Context, bp *Blueprint) error {
	now := time.Now()
	result, err := r.db.NewUpdate().
		Model((*Blueprint)(nil)).
		Where("id = ?", bp.ID).
		Set("description = ?", bp.Description).
		Set("author = ?", bp.Author).
		Set("manifest = ?", bp.Manifest).
		Set("updated_at = ?", now).
		Exec(ctx)
	if err != nil {
		r.log.Error("failed to update blueprint", logger.Error(err))
		return apperror.ErrDatabase.WithInternal(err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperror.ErrNotFound.WithMessage("blueprint not found")
	}

	bp.UpdatedAt = now
	return nil
}

// Delete removes a blueprint row by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.NewDelete().
		Model((*Blueprint)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		r.log.Error("failed to delete blueprint", logger.Error(err))
		return apperror.ErrDatabase.WithInternal(err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperror.ErrNotFound.WithMessage("blueprint not found")
	}

	return nil
}

// ListVersionsByName returns all versions of a blueprint name, ordered by version.
func (r *Repository) ListVersionsByName(ctx context.Context, name string) ([]Blueprint, error) {
	var blueprints []Blueprint
	err := r.db.NewSelect().
		Model(&blueprints).
		Where("name = ?", name).
		Order("version ASC").
		Scan(ctx)
	if err != nil {
		r.log.Error("failed to list blueprint versions", logger.Error(err))
		return nil, apperror.ErrDatabase.WithInternal(err)
	}
	if blueprints == nil {
		return []Blueprint{}, nil
	}
	return blueprints, nil
}

// UpdateStatus transitions a blueprint's status and sets its checksum.
func (r *Repository) UpdateStatus(ctx context.Context, id, status, checksum string) error {
	result, err := r.db.NewUpdate().
		Model((*Blueprint)(nil)).
		Where("id = ?", id).
		Set("status = ?", status).
		Set("checksum = ?", checksum).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)
	if err != nil {
		r.log.Error("failed to update blueprint status", logger.Error(err))
		return apperror.ErrDatabase.WithInternal(err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return apperror.ErrNotFound.WithMessage("blueprint not found")
	}

	return nil
}

// ExistsByNameVersion reports whether a blueprint with the given name and version exists.
// This is a friendly fast-path pre-check; the ON CONFLICT clause in Create is
// the real TOCTOU-safe enforcement.
func (r *Repository) ExistsByNameVersion(ctx context.Context, name, version string) (bool, error) {
	exists, err := r.db.NewSelect().
		Model((*Blueprint)(nil)).
		Where("name = ?", name).
		Where("version = ?", version).
		Exists(ctx)
	if err != nil {
		r.log.Error("failed to check blueprint existence", logger.Error(err))
		return false, apperror.ErrDatabase.WithInternal(err)
	}
	return exists, nil
}
