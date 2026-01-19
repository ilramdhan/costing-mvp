package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ParameterGroup represents a group of parameters
type ParameterGroup struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// MasterParameter represents a parameter definition
type MasterParameter struct {
	Key           string    `json:"key"`
	Label         string    `json:"label"`
	DataType      string    `json:"data_type"`
	DefaultValue  string    `json:"default_value,omitempty"`
	GroupCode     string    `json:"group_code,omitempty"`
	Unit          string    `json:"unit,omitempty"`
	IsRequired    bool      `json:"is_required"`
	SequenceOrder int       `json:"sequence_order"`
	CreatedAt     time.Time `json:"created_at"`
}

// MasterYarn represents a master yarn record
type MasterYarn struct {
	ID          uuid.UUID              `json:"id"`
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	FixedAttrs  map[string]interface{} `json:"fixed_attrs"` // 10 fixed parameters as JSONB
	IsActive    bool                   `json:"is_active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// FixedAttrsJSON returns fixed_attrs as JSON bytes
func (m *MasterYarn) FixedAttrsJSON() ([]byte, error) {
	return json.Marshal(m.FixedAttrs)
}

// YarnVariant represents a child of MasterYarn
type YarnVariant struct {
	ID                uuid.UUID `json:"id"`
	MasterYarnID      uuid.UUID `json:"master_yarn_id"`
	SKU               string    `json:"sku"`
	BatchNo           string    `json:"batch_no,omitempty"`
	RoutingTemplateID uuid.UUID `json:"routing_template_id,omitempty"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ProcessMaster represents a manufacturing process type
type ProcessMaster struct {
	ID              uuid.UUID `json:"id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	DefaultSequence int       `json:"default_sequence"`
	CreatedAt       time.Time `json:"created_at"`
}

// RoutingTemplate represents a combination of processes for a product
type RoutingTemplate struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProcessStep represents a step in a routing with its formula
type ProcessStep struct {
	ID                uuid.UUID `json:"id"`
	RoutingTemplateID uuid.UUID `json:"routing_template_id"`
	ProcessMasterID   uuid.UUID `json:"process_master_id"`
	SequenceOrder     int       `json:"sequence_order"`
	FormulaExpression string    `json:"formula_expression"` // e.g., "(electricity_kwh * 1.5) + labor_cost"
	Description       string    `json:"description,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// VariantProcessCost represents the calculated cost for a variant's process step
type VariantProcessCost struct {
	ID             uuid.UUID              `json:"id"`
	YarnVariantID  uuid.UUID              `json:"yarn_variant_id"`
	ProcessStepID  uuid.UUID              `json:"process_step_id"`
	InputValues    map[string]interface{} `json:"input_values"` // 250 parameters as JSONB
	CalculatedCost float64                `json:"calculated_cost"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// InputValuesJSON returns input_values as JSON bytes
func (v *VariantProcessCost) InputValuesJSON() ([]byte, error) {
	return json.Marshal(v.InputValues)
}

// VariantCostSummary represents the aggregated cost summary for a variant (Read Model)
type VariantCostSummary struct {
	YarnVariantID      uuid.UUID `json:"yarn_variant_id"`
	TotalMaterialCost  float64   `json:"total_material_cost"`
	TotalProcessCost   float64   `json:"total_process_cost"`
	TotalOverhead      float64   `json:"total_overhead"`
	GrandTotal         float64   `json:"grand_total"`
	LastRecalculatedAt time.Time `json:"last_recalculated_at,omitempty"`
	VersionHash        string    `json:"version_hash,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// JobStatus represents the status of a batch job
type JobStatus string

const (
	JobStatusPending   JobStatus = "PENDING"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusCompleted JobStatus = "COMPLETED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusCancelled JobStatus = "CANCELLED"
)

// JobType represents the type of batch job
type JobType string

const (
	JobTypeRecalculateAll     JobType = "RECALCULATE_ALL"
	JobTypeRecalculateMaster  JobType = "RECALCULATE_MASTER"
	JobTypeRecalculateVariant JobType = "RECALCULATE_VARIANT"
	JobTypeImportData         JobType = "IMPORT_DATA"
	JobTypeExportData         JobType = "EXPORT_DATA"
)

// BatchJob represents a background job for large operations
type BatchJob struct {
	ID               uuid.UUID              `json:"id"`
	JobType          JobType                `json:"job_type"`
	Status           JobStatus              `json:"status"`
	TotalRecords     int64                  `json:"total_records"`
	ProcessedRecords int64                  `json:"processed_records"`
	FailedRecords    int64                  `json:"failed_records"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	StartedAt        *time.Time             `json:"started_at,omitempty"`
	FinishedAt       *time.Time             `json:"finished_at,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

// Progress returns the progress percentage
func (b *BatchJob) Progress() float64 {
	if b.TotalRecords == 0 {
		return 0
	}
	return float64(b.ProcessedRecords) / float64(b.TotalRecords) * 100
}

// PriceRate represents a pricing rate for a parameter
type PriceRate struct {
	ID            uuid.UUID  `json:"id"`
	ParameterKey  string     `json:"parameter_key"`
	RateValue     float64    `json:"rate_value"`
	EffectiveDate time.Time  `json:"effective_date"`
	ExpiredDate   *time.Time `json:"expired_date,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
