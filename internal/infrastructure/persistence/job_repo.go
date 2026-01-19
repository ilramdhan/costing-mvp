package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/domain/repository"
)

// batchJobRepo implements repository.BatchJobRepository
type batchJobRepo struct {
	pool *pgxpool.Pool
}

// NewBatchJobRepository creates a new batch job repository
func NewBatchJobRepository(pool *pgxpool.Pool) repository.BatchJobRepository {
	return &batchJobRepo{pool: pool}
}

func (r *batchJobRepo) Create(ctx context.Context, job *entity.BatchJob) error {
	query := `
		INSERT INTO batch_jobs (id, job_type, status, total_records, processed_records, failed_records, metadata, error_message, started_at, finished_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.pool.Exec(ctx, query,
		job.ID, job.JobType, job.Status, job.TotalRecords, job.ProcessedRecords, job.FailedRecords, job.Metadata, job.ErrorMessage, job.StartedAt, job.FinishedAt, job.CreatedAt)
	return err
}

func (r *batchJobRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.BatchJob, error) {
	query := `
		SELECT id, job_type, status, total_records, processed_records, failed_records, metadata, error_message, started_at, finished_at, created_at
		FROM batch_jobs WHERE id = $1
	`
	var job entity.BatchJob
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.JobType, &job.Status, &job.TotalRecords, &job.ProcessedRecords, &job.FailedRecords, &job.Metadata, &job.ErrorMessage, &job.StartedAt, &job.FinishedAt, &job.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *batchJobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.JobStatus, processed, failed int64) error {
	query := `
		UPDATE batch_jobs SET status = $2, processed_records = $3, failed_records = $4
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, status, processed, failed)
	return err
}

func (r *batchJobRepo) UpdateProgress(ctx context.Context, id uuid.UUID, processed, failed int64) error {
	query := `
		UPDATE batch_jobs SET processed_records = processed_records + $2, failed_records = failed_records + $3
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, processed, failed)
	return err
}

func (r *batchJobRepo) Complete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE batch_jobs SET status = $2, finished_at = $3
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, entity.JobStatusCompleted, now)
	return err
}

func (r *batchJobRepo) Fail(ctx context.Context, id uuid.UUID, errorMsg string) error {
	now := time.Now()
	query := `
		UPDATE batch_jobs SET status = $2, error_message = $3, finished_at = $4
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, entity.JobStatusFailed, errorMsg, now)
	return err
}

func (r *batchJobRepo) ListRecent(ctx context.Context, limit int) ([]*entity.BatchJob, error) {
	query := `
		SELECT id, job_type, status, total_records, processed_records, failed_records, metadata, error_message, started_at, finished_at, created_at
		FROM batch_jobs ORDER BY created_at DESC LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*entity.BatchJob
	for rows.Next() {
		var job entity.BatchJob
		if err := rows.Scan(&job.ID, &job.JobType, &job.Status, &job.TotalRecords, &job.ProcessedRecords, &job.FailedRecords, &job.Metadata, &job.ErrorMessage, &job.StartedAt, &job.FinishedAt, &job.CreatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

// processStepRepo implements repository.ProcessStepRepository
type processStepRepo struct {
	pool *pgxpool.Pool
}

// NewProcessStepRepository creates a new process step repository
func NewProcessStepRepository(pool *pgxpool.Pool) repository.ProcessStepRepository {
	return &processStepRepo{pool: pool}
}

func (r *processStepRepo) GetByRoutingID(ctx context.Context, routingID uuid.UUID) ([]*entity.ProcessStep, error) {
	query := `
		SELECT id, routing_template_id, process_master_id, sequence_order, formula_expression, description, created_at
		FROM process_steps WHERE routing_template_id = $1 ORDER BY sequence_order
	`
	rows, err := r.pool.Query(ctx, query, routingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*entity.ProcessStep
	for rows.Next() {
		var s entity.ProcessStep
		if err := rows.Scan(&s.ID, &s.RoutingTemplateID, &s.ProcessMasterID, &s.SequenceOrder, &s.FormulaExpression, &s.Description, &s.CreatedAt); err != nil {
			return nil, err
		}
		steps = append(steps, &s)
	}
	return steps, nil
}

func (r *processStepRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.ProcessStep, error) {
	query := `
		SELECT id, routing_template_id, process_master_id, sequence_order, formula_expression, description, created_at
		FROM process_steps WHERE id = $1
	`
	var s entity.ProcessStep
	err := r.pool.QueryRow(ctx, query, id).Scan(&s.ID, &s.RoutingTemplateID, &s.ProcessMasterID, &s.SequenceOrder, &s.FormulaExpression, &s.Description, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// routingTemplateRepo implements repository.RoutingTemplateRepository
type routingTemplateRepo struct {
	pool *pgxpool.Pool
}

// NewRoutingTemplateRepository creates a new routing template repository
func NewRoutingTemplateRepository(pool *pgxpool.Pool) repository.RoutingTemplateRepository {
	return &routingTemplateRepo{pool: pool}
}

func (r *routingTemplateRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.RoutingTemplate, error) {
	query := `SELECT id, name, description, is_active, created_at FROM routing_templates WHERE id = $1`
	var t entity.RoutingTemplate
	err := r.pool.QueryRow(ctx, query, id).Scan(&t.ID, &t.Name, &t.Description, &t.IsActive, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *routingTemplateRepo) List(ctx context.Context) ([]*entity.RoutingTemplate, error) {
	query := `SELECT id, name, description, is_active, created_at FROM routing_templates WHERE is_active = true ORDER BY name`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*entity.RoutingTemplate
	for rows.Next() {
		var t entity.RoutingTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.IsActive, &t.CreatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, &t)
	}
	return templates, nil
}

func (r *routingTemplateRepo) Create(ctx context.Context, template *entity.RoutingTemplate) error {
	query := `INSERT INTO routing_templates (id, name, description, is_active, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, template.ID, template.Name, template.Description, template.IsActive, template.CreatedAt)
	return err
}

// processMasterRepo implements repository.ProcessMasterRepository
type processMasterRepo struct {
	pool *pgxpool.Pool
}

// NewProcessMasterRepository creates a new process master repository
func NewProcessMasterRepository(pool *pgxpool.Pool) repository.ProcessMasterRepository {
	return &processMasterRepo{pool: pool}
}

func (r *processMasterRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.ProcessMaster, error) {
	query := `SELECT id, code, name, description, default_sequence, created_at FROM process_masters WHERE id = $1`
	var p entity.ProcessMaster
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.DefaultSequence, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *processMasterRepo) List(ctx context.Context) ([]*entity.ProcessMaster, error) {
	query := `SELECT id, code, name, description, default_sequence, created_at FROM process_masters ORDER BY default_sequence`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var processes []*entity.ProcessMaster
	for rows.Next() {
		var p entity.ProcessMaster
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.DefaultSequence, &p.CreatedAt); err != nil {
			return nil, err
		}
		processes = append(processes, &p)
	}
	return processes, nil
}

func (r *processMasterRepo) Create(ctx context.Context, process *entity.ProcessMaster) error {
	query := `INSERT INTO process_masters (id, code, name, description, default_sequence, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, query, process.ID, process.Code, process.Name, process.Description, process.DefaultSequence, process.CreatedAt)
	return err
}

func (r *processMasterRepo) CreateBatch(ctx context.Context, processes []*entity.ProcessMaster) (int64, error) {
	columns := []string{"id", "code", "name", "description", "default_sequence", "created_at"}
	rows := make([][]interface{}, len(processes))
	for i, p := range processes {
		rows[i] = []interface{}{p.ID, p.Code, p.Name, p.Description, p.DefaultSequence, p.CreatedAt}
	}
	return r.pool.CopyFrom(ctx, pgx.Identifier{"process_masters"}, columns, pgx.CopyFromRows(rows))
}
