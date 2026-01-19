package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
)

// MasterYarnRepository defines the interface for master yarn operations
type MasterYarnRepository interface {
	// Create creates a new master yarn
	Create(ctx context.Context, yarn *entity.MasterYarn) error
	// CreateBatch creates multiple master yarns using COPY protocol
	CreateBatch(ctx context.Context, yarns []*entity.MasterYarn) (int64, error)
	// GetByID retrieves a master yarn by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.MasterYarn, error)
	// GetByCode retrieves a master yarn by code
	GetByCode(ctx context.Context, code string) (*entity.MasterYarn, error)
	// List retrieves master yarns with pagination
	List(ctx context.Context, limit, offset int) ([]*entity.MasterYarn, error)
	// Count returns the total count of master yarns
	Count(ctx context.Context) (int64, error)
	// Update updates a master yarn
	Update(ctx context.Context, yarn *entity.MasterYarn) error
	// Delete deletes a master yarn
	Delete(ctx context.Context, id uuid.UUID) error
}

// YarnVariantRepository defines the interface for yarn variant operations
type YarnVariantRepository interface {
	// Create creates a new yarn variant
	Create(ctx context.Context, variant *entity.YarnVariant) error
	// CreateBatch creates multiple variants using COPY protocol
	CreateBatch(ctx context.Context, variants []*entity.YarnVariant) (int64, error)
	// GetByID retrieves a variant by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.YarnVariant, error)
	// GetBySKU retrieves a variant by SKU
	GetBySKU(ctx context.Context, sku string) (*entity.YarnVariant, error)
	// ListByMasterID retrieves variants by master yarn ID
	ListByMasterID(ctx context.Context, masterID uuid.UUID, limit, offset int) ([]*entity.YarnVariant, error)
	// ListIDs retrieves variant IDs with pagination (for batch processing)
	ListIDs(ctx context.Context, limit, offset int) ([]uuid.UUID, error)
	// Count returns the total count of variants
	Count(ctx context.Context) (int64, error)
	// CountByMasterID returns the count of variants for a master
	CountByMasterID(ctx context.Context, masterID uuid.UUID) (int64, error)
}

// ProcessStepRepository defines the interface for process step operations
type ProcessStepRepository interface {
	// GetByRoutingID retrieves all steps for a routing template
	GetByRoutingID(ctx context.Context, routingID uuid.UUID) ([]*entity.ProcessStep, error)
	// GetByID retrieves a step by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.ProcessStep, error)
}

// VariantProcessCostRepository defines the interface for variant process cost operations
type VariantProcessCostRepository interface {
	// Upsert creates or updates a variant process cost
	Upsert(ctx context.Context, cost *entity.VariantProcessCost) error
	// UpsertBatch creates or updates multiple costs using COPY protocol
	UpsertBatch(ctx context.Context, costs []*entity.VariantProcessCost) (int64, error)
	// GetByVariantID retrieves all costs for a variant
	GetByVariantID(ctx context.Context, variantID uuid.UUID) ([]*entity.VariantProcessCost, error)
	// DeleteByVariantID deletes all costs for a variant
	DeleteByVariantID(ctx context.Context, variantID uuid.UUID) error
}

// VariantCostSummaryRepository defines the interface for cost summary operations
type VariantCostSummaryRepository interface {
	// Upsert creates or updates a cost summary
	Upsert(ctx context.Context, summary *entity.VariantCostSummary) error
	// UpsertBatch creates or updates multiple summaries
	UpsertBatch(ctx context.Context, summaries []*entity.VariantCostSummary) (int64, error)
	// GetByVariantID retrieves a summary by variant ID
	GetByVariantID(ctx context.Context, variantID uuid.UUID) (*entity.VariantCostSummary, error)
	// List retrieves summaries with pagination
	List(ctx context.Context, limit, offset int) ([]*entity.VariantCostSummary, error)
}

// BatchJobRepository defines the interface for batch job operations
type BatchJobRepository interface {
	// Create creates a new batch job
	Create(ctx context.Context, job *entity.BatchJob) error
	// GetByID retrieves a job by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.BatchJob, error)
	// UpdateStatus updates a job's status and progress
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.JobStatus, processed, failed int64) error
	// UpdateProgress updates a job's progress atomically
	UpdateProgress(ctx context.Context, id uuid.UUID, processed, failed int64) error
	// Complete marks a job as completed
	Complete(ctx context.Context, id uuid.UUID) error
	// Fail marks a job as failed
	Fail(ctx context.Context, id uuid.UUID, errorMsg string) error
	// ListRecent retrieves recent jobs
	ListRecent(ctx context.Context, limit int) ([]*entity.BatchJob, error)
}

// RoutingTemplateRepository defines the interface for routing template operations
type RoutingTemplateRepository interface {
	// GetByID retrieves a routing template by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.RoutingTemplate, error)
	// List retrieves all active routing templates
	List(ctx context.Context) ([]*entity.RoutingTemplate, error)
	// Create creates a new routing template
	Create(ctx context.Context, template *entity.RoutingTemplate) error
}

// ProcessMasterRepository defines the interface for process master operations
type ProcessMasterRepository interface {
	// GetByID retrieves a process master by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.ProcessMaster, error)
	// List retrieves all process masters
	List(ctx context.Context) ([]*entity.ProcessMaster, error)
	// Create creates a new process master
	Create(ctx context.Context, process *entity.ProcessMaster) error
	// CreateBatch creates multiple processes
	CreateBatch(ctx context.Context, processes []*entity.ProcessMaster) (int64, error)
}

// PriceRateRepository defines the interface for price rate operations
type PriceRateRepository interface {
	// GetCurrentRate retrieves the current rate for a parameter
	GetCurrentRate(ctx context.Context, parameterKey string) (*entity.PriceRate, error)
	// GetAllCurrentRates retrieves all current rates
	GetAllCurrentRates(ctx context.Context) (map[string]float64, error)
	// Create creates a new price rate
	Create(ctx context.Context, rate *entity.PriceRate) error
	// CreateBatch creates multiple rates
	CreateBatch(ctx context.Context, rates []*entity.PriceRate) (int64, error)
}
