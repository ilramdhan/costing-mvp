package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/domain/repository"
)

// variantProcessCostRepo implements repository.VariantProcessCostRepository
type variantProcessCostRepo struct {
	pool *pgxpool.Pool
}

// NewVariantProcessCostRepository creates a new variant process cost repository
func NewVariantProcessCostRepository(pool *pgxpool.Pool) repository.VariantProcessCostRepository {
	return &variantProcessCostRepo{pool: pool}
}

func (r *variantProcessCostRepo) Upsert(ctx context.Context, cost *entity.VariantProcessCost) error {
	query := `
		INSERT INTO variant_process_costs (id, yarn_variant_id, process_step_id, input_values, calculated_cost, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id, yarn_variant_id) DO UPDATE SET
			input_values = EXCLUDED.input_values,
			calculated_cost = EXCLUDED.calculated_cost,
			updated_at = EXCLUDED.updated_at
	`
	inputValues, _ := cost.InputValuesJSON()
	_, err := r.pool.Exec(ctx, query,
		cost.ID, cost.YarnVariantID, cost.ProcessStepID, inputValues, cost.CalculatedCost, cost.UpdatedAt)
	return err
}

// UpsertBatch uses PostgreSQL COPY protocol for high-performance bulk inserts
// For updates, we use a temp table approach
func (r *variantProcessCostRepo) UpsertBatch(ctx context.Context, costs []*entity.VariantProcessCost) (int64, error) {
	if len(costs) == 0 {
		return 0, nil
	}

	// Use a transaction for atomic operations
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create temp table
	tempTable := fmt.Sprintf("temp_vpc_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		CREATE TEMP TABLE %s (
			id UUID,
			yarn_variant_id UUID,
			process_step_id UUID,
			input_values JSONB,
			calculated_cost DECIMAL(18,6),
			updated_at TIMESTAMPTZ
		) ON COMMIT DROP
	`, tempTable))
	if err != nil {
		return 0, fmt.Errorf("failed to create temp table: %w", err)
	}

	// COPY data to temp table
	columns := []string{"id", "yarn_variant_id", "process_step_id", "input_values", "calculated_cost", "updated_at"}
	rows := make([][]interface{}, len(costs))
	for i, c := range costs {
		inputValues, _ := json.Marshal(c.InputValues)
		rows[i] = []interface{}{
			c.ID, c.YarnVariantID, c.ProcessStepID, inputValues, c.CalculatedCost, c.UpdatedAt,
		}
	}

	copyCount, err := tx.CopyFrom(ctx, pgx.Identifier{tempTable}, columns, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("failed to copy to temp table: %w", err)
	}

	// Upsert from temp table to main table
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO variant_process_costs (id, yarn_variant_id, process_step_id, input_values, calculated_cost, updated_at)
		SELECT id, yarn_variant_id, process_step_id, input_values, calculated_cost, updated_at FROM %s
		ON CONFLICT (id, yarn_variant_id) DO UPDATE SET
			input_values = EXCLUDED.input_values,
			calculated_cost = EXCLUDED.calculated_cost,
			updated_at = EXCLUDED.updated_at
	`, tempTable))
	if err != nil {
		return 0, fmt.Errorf("failed to upsert from temp table: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return copyCount, nil
}

func (r *variantProcessCostRepo) GetByVariantID(ctx context.Context, variantID uuid.UUID) ([]*entity.VariantProcessCost, error) {
	query := `
		SELECT id, yarn_variant_id, process_step_id, input_values, calculated_cost, updated_at
		FROM variant_process_costs WHERE yarn_variant_id = $1
	`
	rows, err := r.pool.Query(ctx, query, variantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var costs []*entity.VariantProcessCost
	for rows.Next() {
		var c entity.VariantProcessCost
		if err := rows.Scan(&c.ID, &c.YarnVariantID, &c.ProcessStepID, &c.InputValues, &c.CalculatedCost, &c.UpdatedAt); err != nil {
			return nil, err
		}
		costs = append(costs, &c)
	}
	return costs, nil
}

func (r *variantProcessCostRepo) DeleteByVariantID(ctx context.Context, variantID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM variant_process_costs WHERE yarn_variant_id = $1", variantID)
	return err
}

// variantCostSummaryRepo implements repository.VariantCostSummaryRepository
type variantCostSummaryRepo struct {
	pool *pgxpool.Pool
}

// NewVariantCostSummaryRepository creates a new variant cost summary repository
func NewVariantCostSummaryRepository(pool *pgxpool.Pool) repository.VariantCostSummaryRepository {
	return &variantCostSummaryRepo{pool: pool}
}

func (r *variantCostSummaryRepo) Upsert(ctx context.Context, summary *entity.VariantCostSummary) error {
	query := `
		INSERT INTO variant_cost_summaries (yarn_variant_id, total_material_cost, total_process_cost, total_overhead, grand_total, last_recalculated_at, version_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (yarn_variant_id) DO UPDATE SET
			total_material_cost = EXCLUDED.total_material_cost,
			total_process_cost = EXCLUDED.total_process_cost,
			total_overhead = EXCLUDED.total_overhead,
			grand_total = EXCLUDED.grand_total,
			last_recalculated_at = EXCLUDED.last_recalculated_at,
			version_hash = EXCLUDED.version_hash
	`
	_, err := r.pool.Exec(ctx, query,
		summary.YarnVariantID, summary.TotalMaterialCost, summary.TotalProcessCost, summary.TotalOverhead, summary.GrandTotal, summary.LastRecalculatedAt, summary.VersionHash)
	return err
}

func (r *variantCostSummaryRepo) UpsertBatch(ctx context.Context, summaries []*entity.VariantCostSummary) (int64, error) {
	if len(summaries) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	tempTable := fmt.Sprintf("temp_vcs_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		CREATE TEMP TABLE %s (
			yarn_variant_id UUID,
			total_material_cost DECIMAL(18,6),
			total_process_cost DECIMAL(18,6),
			total_overhead DECIMAL(18,6),
			grand_total DECIMAL(18,6),
			last_recalculated_at TIMESTAMPTZ,
			version_hash VARCHAR(64)
		) ON COMMIT DROP
	`, tempTable))
	if err != nil {
		return 0, err
	}

	columns := []string{"yarn_variant_id", "total_material_cost", "total_process_cost", "total_overhead", "grand_total", "last_recalculated_at", "version_hash"}
	rows := make([][]interface{}, len(summaries))
	for i, s := range summaries {
		rows[i] = []interface{}{
			s.YarnVariantID, s.TotalMaterialCost, s.TotalProcessCost, s.TotalOverhead, s.GrandTotal, s.LastRecalculatedAt, s.VersionHash,
		}
	}

	copyCount, err := tx.CopyFrom(ctx, pgx.Identifier{tempTable}, columns, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO variant_cost_summaries (yarn_variant_id, total_material_cost, total_process_cost, total_overhead, grand_total, last_recalculated_at, version_hash)
		SELECT yarn_variant_id, total_material_cost, total_process_cost, total_overhead, grand_total, last_recalculated_at, version_hash FROM %s
		ON CONFLICT (yarn_variant_id) DO UPDATE SET
			total_material_cost = EXCLUDED.total_material_cost,
			total_process_cost = EXCLUDED.total_process_cost,
			total_overhead = EXCLUDED.total_overhead,
			grand_total = EXCLUDED.grand_total,
			last_recalculated_at = EXCLUDED.last_recalculated_at,
			version_hash = EXCLUDED.version_hash
	`, tempTable))
	if err != nil {
		return 0, err
	}

	return copyCount, tx.Commit(ctx)
}

func (r *variantCostSummaryRepo) GetByVariantID(ctx context.Context, variantID uuid.UUID) (*entity.VariantCostSummary, error) {
	query := `
		SELECT yarn_variant_id, total_material_cost, total_process_cost, total_overhead, grand_total, last_recalculated_at, version_hash, created_at, updated_at
		FROM variant_cost_summaries WHERE yarn_variant_id = $1
	`
	var s entity.VariantCostSummary
	err := r.pool.QueryRow(ctx, query, variantID).Scan(
		&s.YarnVariantID, &s.TotalMaterialCost, &s.TotalProcessCost, &s.TotalOverhead, &s.GrandTotal, &s.LastRecalculatedAt, &s.VersionHash, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *variantCostSummaryRepo) List(ctx context.Context, limit, offset int) ([]*entity.VariantCostSummary, error) {
	query := `
		SELECT yarn_variant_id, total_material_cost, total_process_cost, total_overhead, grand_total, last_recalculated_at, version_hash, created_at, updated_at
		FROM variant_cost_summaries ORDER BY updated_at DESC LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*entity.VariantCostSummary
	for rows.Next() {
		var s entity.VariantCostSummary
		if err := rows.Scan(&s.YarnVariantID, &s.TotalMaterialCost, &s.TotalProcessCost, &s.TotalOverhead, &s.GrandTotal, &s.LastRecalculatedAt, &s.VersionHash, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		summaries = append(summaries, &s)
	}
	return summaries, nil
}
