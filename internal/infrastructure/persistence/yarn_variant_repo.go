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

// yarnVariantRepo implements repository.YarnVariantRepository
type yarnVariantRepo struct {
	pool *pgxpool.Pool
}

// NewYarnVariantRepository creates a new yarn variant repository
func NewYarnVariantRepository(pool *pgxpool.Pool) repository.YarnVariantRepository {
	return &yarnVariantRepo{pool: pool}
}

func (r *yarnVariantRepo) Create(ctx context.Context, variant *entity.YarnVariant) error {
	query := `
		INSERT INTO yarn_variants (id, master_yarn_id, sku, batch_no, routing_template_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		variant.ID, variant.MasterYarnID, variant.SKU, variant.BatchNo, variant.RoutingTemplateID, variant.IsActive, variant.CreatedAt, variant.UpdatedAt)
	return err
}

// CreateBatch uses PostgreSQL COPY protocol for high-performance bulk inserts
func (r *yarnVariantRepo) CreateBatch(ctx context.Context, variants []*entity.YarnVariant) (int64, error) {
	columns := []string{"id", "master_yarn_id", "sku", "batch_no", "routing_template_id", "is_active", "created_at", "updated_at"}

	rows := make([][]interface{}, len(variants))
	for i, v := range variants {
		var routingID interface{}
		if v.RoutingTemplateID != uuid.Nil {
			routingID = v.RoutingTemplateID
		}
		rows[i] = []interface{}{
			v.ID, v.MasterYarnID, v.SKU, v.BatchNo, routingID, v.IsActive, v.CreatedAt, v.UpdatedAt,
		}
	}

	copyCount, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"yarn_variants"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy yarn variants: %w", err)
	}

	return copyCount, nil
}

func (r *yarnVariantRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.YarnVariant, error) {
	query := `
		SELECT id, master_yarn_id, sku, batch_no, routing_template_id, is_active, created_at, updated_at
		FROM yarn_variants WHERE id = $1
	`
	var v entity.YarnVariant
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&v.ID, &v.MasterYarnID, &v.SKU, &v.BatchNo, &v.RoutingTemplateID, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *yarnVariantRepo) GetBySKU(ctx context.Context, sku string) (*entity.YarnVariant, error) {
	query := `
		SELECT id, master_yarn_id, sku, batch_no, routing_template_id, is_active, created_at, updated_at
		FROM yarn_variants WHERE sku = $1
	`
	var v entity.YarnVariant
	err := r.pool.QueryRow(ctx, query, sku).Scan(
		&v.ID, &v.MasterYarnID, &v.SKU, &v.BatchNo, &v.RoutingTemplateID, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *yarnVariantRepo) ListByMasterID(ctx context.Context, masterID uuid.UUID, limit, offset int) ([]*entity.YarnVariant, error) {
	query := `
		SELECT id, master_yarn_id, sku, batch_no, routing_template_id, is_active, created_at, updated_at
		FROM yarn_variants WHERE master_yarn_id = $1 ORDER BY created_at LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, masterID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var variants []*entity.YarnVariant
	for rows.Next() {
		var v entity.YarnVariant
		if err := rows.Scan(&v.ID, &v.MasterYarnID, &v.SKU, &v.BatchNo, &v.RoutingTemplateID, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		variants = append(variants, &v)
	}
	return variants, nil
}

// ListIDs retrieves variant IDs in batches for worker processing
func (r *yarnVariantRepo) ListIDs(ctx context.Context, limit, offset int) ([]uuid.UUID, error) {
	query := `SELECT id FROM yarn_variants WHERE is_active = true ORDER BY id LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ListWithRouting retrieves variants with routing IDs (optimized - only fetches id and routing_template_id)
func (r *yarnVariantRepo) ListWithRouting(ctx context.Context, limit, offset int) ([]*entity.YarnVariant, error) {
	query := `SELECT id, routing_template_id FROM yarn_variants WHERE is_active = true ORDER BY id LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]*entity.YarnVariant, 0, limit)
	for rows.Next() {
		var v entity.YarnVariant
		if err := rows.Scan(&v.ID, &v.RoutingTemplateID); err != nil {
			return nil, err
		}
		variants = append(variants, &v)
	}
	return variants, nil
}

// ListUniqueRoutingIDs retrieves all unique routing template IDs (for caching)
func (r *yarnVariantRepo) ListUniqueRoutingIDs(ctx context.Context) ([]uuid.UUID, error) {
	query := `SELECT DISTINCT routing_template_id FROM yarn_variants WHERE routing_template_id IS NOT NULL`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *yarnVariantRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM yarn_variants").Scan(&count)
	return count, err
}

func (r *yarnVariantRepo) CountByMasterID(ctx context.Context, masterID uuid.UUID) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM yarn_variants WHERE master_yarn_id = $1", masterID).Scan(&count)
	return count, err
}
