package costing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/domain/repository"
	"github.com/ilramdhan/costing-mvp/pkg/formula"
)

// CalculationEngine handles cost calculations
type CalculationEngine struct {
	variantRepo     repository.YarnVariantRepository
	processStepRepo repository.ProcessStepRepository
	costRepo        repository.VariantProcessCostRepository
	summaryRepo     repository.VariantCostSummaryRepository
	formulaParser   *formula.Parser
}

// NewCalculationEngine creates a new calculation engine
func NewCalculationEngine(
	variantRepo repository.YarnVariantRepository,
	processStepRepo repository.ProcessStepRepository,
	costRepo repository.VariantProcessCostRepository,
	summaryRepo repository.VariantCostSummaryRepository,
) *CalculationEngine {
	return &CalculationEngine{
		variantRepo:     variantRepo,
		processStepRepo: processStepRepo,
		costRepo:        costRepo,
		summaryRepo:     summaryRepo,
		formulaParser:   formula.NewParser(),
	}
}

// CalculateVariant calculates costs for a single variant
func (e *CalculationEngine) CalculateVariant(ctx context.Context, variantID uuid.UUID, inputParams map[string]interface{}) (*entity.VariantCostSummary, error) {
	// Get variant
	variant, err := e.variantRepo.GetByID(ctx, variantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get variant: %w", err)
	}

	// Get process steps for routing
	steps, err := e.processStepRepo.GetByRoutingID(ctx, variant.RoutingTemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get process steps: %w", err)
	}

	var totalProcessCost float64
	var processCosts []*entity.VariantProcessCost
	now := time.Now()

	// Calculate each step
	for _, step := range steps {
		cost, err := e.formulaParser.Evaluate(step.FormulaExpression, inputParams)
		if err != nil {
			log.Printf("Warning: failed to evaluate formula for step %s: %v", step.ID, err)
			cost = 0
		}

		processCosts = append(processCosts, &entity.VariantProcessCost{
			ID:             uuid.New(),
			YarnVariantID:  variantID,
			ProcessStepID:  step.ID,
			InputValues:    inputParams,
			CalculatedCost: cost,
			UpdatedAt:      now,
		})

		totalProcessCost += cost
	}

	// Calculate summary
	materialCost := getFloatParam(inputParams, "material_cost", 0)
	overhead := totalProcessCost * getFloatParam(inputParams, "overhead_percentage", 0.1)

	// Generate version hash for change detection
	paramsJSON, _ := json.Marshal(inputParams)
	hash := sha256.Sum256(paramsJSON)

	summary := &entity.VariantCostSummary{
		YarnVariantID:      variantID,
		TotalMaterialCost:  materialCost,
		TotalProcessCost:   totalProcessCost,
		TotalOverhead:      overhead,
		GrandTotal:         materialCost + totalProcessCost + overhead,
		LastRecalculatedAt: now,
		VersionHash:        hex.EncodeToString(hash[:]),
	}

	return summary, nil
}

func getFloatParam(params map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		}
	}
	return defaultVal
}

// WorkerPool manages concurrent calculation workers
type WorkerPool struct {
	engine      *CalculationEngine
	variantRepo repository.YarnVariantRepository
	summaryRepo repository.VariantCostSummaryRepository
	jobRepo     repository.BatchJobRepository
	workerCount int
	batchSize   int
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	engine *CalculationEngine,
	variantRepo repository.YarnVariantRepository,
	summaryRepo repository.VariantCostSummaryRepository,
	jobRepo repository.BatchJobRepository,
	workerCount, batchSize int,
) *WorkerPool {
	return &WorkerPool{
		engine:      engine,
		variantRepo: variantRepo,
		summaryRepo: summaryRepo,
		jobRepo:     jobRepo,
		workerCount: workerCount,
		batchSize:   batchSize,
	}
}

// RecalculateAll recalculates costs for all variants
func (wp *WorkerPool) RecalculateAll(ctx context.Context, jobID uuid.UUID, baseParams map[string]interface{}) error {
	// Get total count
	totalCount, err := wp.variantRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count variants: %w", err)
	}

	// Update job with total
	wp.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusRunning, 0, 0)

	// Create channels
	idChan := make(chan uuid.UUID, wp.batchSize*2)
	resultChan := make(chan *entity.VariantCostSummary, wp.batchSize*2)
	errChan := make(chan error, 1)

	var processedCount int64
	var failedCount int64

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < wp.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for variantID := range idChan {
				summary, err := wp.engine.CalculateVariant(ctx, variantID, baseParams)
				if err != nil {
					log.Printf("Worker %d: failed to calculate variant %s: %v", workerID, variantID, err)
					atomic.AddInt64(&failedCount, 1)
					continue
				}
				resultChan <- summary
			}
		}(i)
	}

	// Start result collector
	var resultWg sync.WaitGroup
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		buffer := make([]*entity.VariantCostSummary, 0, wp.batchSize)

		for summary := range resultChan {
			buffer = append(buffer, summary)

			if len(buffer) >= wp.batchSize {
				if _, err := wp.summaryRepo.UpsertBatch(ctx, buffer); err != nil {
					log.Printf("Failed to upsert batch: %v", err)
				}
				atomic.AddInt64(&processedCount, int64(len(buffer)))

				// Update job progress periodically
				wp.jobRepo.UpdateProgress(ctx, jobID, int64(len(buffer)), 0)

				buffer = buffer[:0]
			}
		}

		// Flush remaining
		if len(buffer) > 0 {
			if _, err := wp.summaryRepo.UpsertBatch(ctx, buffer); err != nil {
				log.Printf("Failed to upsert final batch: %v", err)
			}
			atomic.AddInt64(&processedCount, int64(len(buffer)))
		}
	}()

	// Dispatcher: fetch IDs and send to workers
	go func() {
		defer close(idChan)
		offset := 0
		for {
			ids, err := wp.variantRepo.ListIDs(ctx, wp.batchSize, offset)
			if err != nil {
				errChan <- fmt.Errorf("failed to list variant IDs: %w", err)
				return
			}
			if len(ids) == 0 {
				break
			}
			for _, id := range ids {
				select {
				case <-ctx.Done():
					return
				case idChan <- id:
				}
			}
			offset += len(ids)
		}
	}()

	// Wait for workers to finish
	wg.Wait()
	close(resultChan)

	// Wait for result collector
	resultWg.Wait()

	// Complete job
	if err := wp.jobRepo.Complete(ctx, jobID); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("Recalculation complete: processed=%d, failed=%d, total=%d", processedCount, failedCount, totalCount)
	return nil
}
