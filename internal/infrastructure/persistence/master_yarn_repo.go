package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/domain/repository"
)

// masterYarnRepo implements repository.MasterYarnRepository
type masterYarnRepo struct {
	pool *pgxpool.Pool
}

// NewMasterYarnRepository creates a new master yarn repository
func NewMasterYarnRepository(pool *pgxpool.Pool) repository.MasterYarnRepository {
	return &masterYarnRepo{pool: pool}
}

func (r *masterYarnRepo) Create(ctx context.Context, yarn *entity.MasterYarn) error {
	query := `
		INSERT INTO master_yarns (id, code, name, description, fixed_attrs, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	fixedAttrs, _ := yarn.FixedAttrsJSON()
	_, err := r.pool.Exec(ctx, query,
		yarn.ID, yarn.Code, yarn.Name, yarn.Description, fixedAttrs, yarn.IsActive, yarn.CreatedAt, yarn.UpdatedAt)
	return err
}

// CreateBatch uses PostgreSQL COPY protocol for high-performance bulk inserts
func (r *masterYarnRepo) CreateBatch(ctx context.Context, yarns []*entity.MasterYarn) (int64, error) {
	columns := []string{"id", "code", "name", "description", "fixed_attrs", "is_active", "created_at", "updated_at"}

	rows := make([][]interface{}, len(yarns))
	for i, yarn := range yarns {
		fixedAttrs, _ := yarn.FixedAttrsJSON()
		rows[i] = []interface{}{
			yarn.ID, yarn.Code, yarn.Name, yarn.Description, fixedAttrs, yarn.IsActive, yarn.CreatedAt, yarn.UpdatedAt,
		}
	}

	copyCount, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"master_yarns"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy master yarns: %w", err)
	}

	return copyCount, nil
}

func (r *masterYarnRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.MasterYarn, error) {
	query := `
		SELECT id, code, name, description, fixed_attrs, is_active, created_at, updated_at
		FROM master_yarns WHERE id = $1
	`
	var yarn entity.MasterYarn
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&yarn.ID, &yarn.Code, &yarn.Name, &yarn.Description, &yarn.FixedAttrs, &yarn.IsActive, &yarn.CreatedAt, &yarn.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &yarn, nil
}

func (r *masterYarnRepo) GetByCode(ctx context.Context, code string) (*entity.MasterYarn, error) {
	query := `
		SELECT id, code, name, description, fixed_attrs, is_active, created_at, updated_at
		FROM master_yarns WHERE code = $1
	`
	var yarn entity.MasterYarn
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&yarn.ID, &yarn.Code, &yarn.Name, &yarn.Description, &yarn.FixedAttrs, &yarn.IsActive, &yarn.CreatedAt, &yarn.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &yarn, nil
}

func (r *masterYarnRepo) List(ctx context.Context, limit, offset int) ([]*entity.MasterYarn, error) {
	query := `
		SELECT id, code, name, description, fixed_attrs, is_active, created_at, updated_at
		FROM master_yarns
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var yarns []*entity.MasterYarn
	for rows.Next() {
		var yarn entity.MasterYarn
		if err := rows.Scan(&yarn.ID, &yarn.Code, &yarn.Name, &yarn.Description, &yarn.FixedAttrs, &yarn.IsActive, &yarn.CreatedAt, &yarn.UpdatedAt); err != nil {
			return nil, err
		}
		yarns = append(yarns, &yarn)
	}
	return yarns, nil
}

func (r *masterYarnRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM master_yarns").Scan(&count)
	return count, err
}

func (r *masterYarnRepo) Update(ctx context.Context, yarn *entity.MasterYarn) error {
	query := `
		UPDATE master_yarns SET code = $2, name = $3, description = $4, fixed_attrs = $5, is_active = $6, updated_at = NOW()
		WHERE id = $1
	`
	fixedAttrs, _ := yarn.FixedAttrsJSON()
	_, err := r.pool.Exec(ctx, query, yarn.ID, yarn.Code, yarn.Name, yarn.Description, fixedAttrs, yarn.IsActive)
	return err
}

func (r *masterYarnRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM master_yarns WHERE id = $1", id)
	return err
}
