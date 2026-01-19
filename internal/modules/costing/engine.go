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

// CalculateVariantFast calculates costs using cached process steps (no DB lookup)
func (e *CalculationEngine) CalculateVariantFast(variantID uuid.UUID, steps []*entity.ProcessStep, inputParams map[string]interface{}) *entity.VariantCostSummary {
	var totalProcessCost float64
	now := time.Now()

	// Calculate each step
	for _, step := range steps {
		cost, err := e.formulaParser.Evaluate(step.FormulaExpression, inputParams)
		if err != nil {
			cost = 0
		}
		totalProcessCost += cost
	}

	// Calculate summary
	materialCost := getFloatParam(inputParams, "material_cost", 0)
	overhead := totalProcessCost * getFloatParam(inputParams, "overhead_percentage", 0.1)

	// Generate version hash for change detection
	paramsJSON, _ := json.Marshal(inputParams)
	hash := sha256.Sum256(paramsJSON)

	return &entity.VariantCostSummary{
		YarnVariantID:      variantID,
		TotalMaterialCost:  materialCost,
		TotalProcessCost:   totalProcessCost,
		TotalOverhead:      overhead,
		GrandTotal:         materialCost + totalProcessCost + overhead,
		LastRecalculatedAt: now,
		VersionHash:        hex.EncodeToString(hash[:]),
	}
}

// CalculateVariant calculates costs for a single variant (with DB lookup - slower)
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

	return e.CalculateVariantFast(variantID, steps, inputParams), nil
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

// RecalculateAll recalculates costs for all variants with optimized batch processing
func (wp *WorkerPool) RecalculateAll(ctx context.Context, jobID uuid.UUID, baseParams map[string]interface{}) error {
	startTime := time.Now()

	// Get total count
	totalCount, err := wp.variantRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count variants: %w", err)
	}

	// Pre-fetch ALL routing templates and their process steps (cached for entire run)
	log.Println("Pre-loading routing templates and process steps...")
	routingStepsCache, err := wp.loadRoutingStepsCache(ctx)
	if err != nil {
		return fmt.Errorf("failed to load routing cache: %w", err)
	}
	log.Printf("Loaded %d routing templates into cache", len(routingStepsCache))

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          TEXTILE COSTING ENGINE - RECALCULATION               ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	log.Printf("Job ID:     %s", jobID)
	log.Printf("Workers:    %d", wp.workerCount)
	log.Printf("Batch Size: %d", wp.batchSize)
	log.Printf("Total Variants: %d", totalCount)
	log.Printf("Routing Cache: %d templates", len(routingStepsCache))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Update job with total
	wp.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusRunning, 0, 0)

	// Create channels - use variant with routing ID to avoid DB lookup
	type variantWork struct {
		ID        uuid.UUID
		RoutingID uuid.UUID
	}
	workChan := make(chan variantWork, wp.batchSize*2)
	resultChan := make(chan *entity.VariantCostSummary, wp.batchSize*2)

	var processedCount int64
	var failedCount int64

	// Progress reporter goroutine
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				processed := atomic.LoadInt64(&processedCount)
				failed := atomic.LoadInt64(&failedCount)
				elapsed := time.Since(startTime)
				if elapsed.Seconds() > 0 && processed > 0 {
					rate := float64(processed) / elapsed.Seconds()
					remaining := float64(totalCount-processed) / rate
					log.Printf("Progress: %d/%d (%.1f%%) | Rate: %.0f/s | Failed: %d | ETA: %v",
						processed, totalCount, float64(processed)/float64(totalCount)*100,
						rate, failed, time.Duration(remaining)*time.Second)
				}
			}
		}
	}()

	// Start workers - use cached steps, no DB query per variant!
	var wg sync.WaitGroup
	for i := 0; i < wp.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for work := range workChan {
				steps, ok := routingStepsCache[work.RoutingID]
				if !ok || len(steps) == 0 {
					atomic.AddInt64(&failedCount, 1)
					continue
				}
				summary := wp.engine.CalculateVariantFast(work.ID, steps, baseParams)
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

	// Dispatcher: fetch variant IDs WITH routing IDs in batches
	go func() {
		defer close(workChan)
		offset := 0
		for {
			variants, err := wp.variantRepo.ListWithRouting(ctx, wp.batchSize, offset)
			if err != nil {
				log.Printf("Failed to list variants: %v", err)
				return
			}
			if len(variants) == 0 {
				break
			}
			for _, v := range variants {
				select {
				case <-ctx.Done():
					return
				case workChan <- variantWork{ID: v.ID, RoutingID: v.RoutingTemplateID}:
				}
			}
			offset += len(variants)
		}
	}()

	// Wait for workers to finish
	wg.Wait()
	close(resultChan)

	// Wait for result collector
	resultWg.Wait()

	// Stop progress reporter
	close(progressDone)

	// Calculate final metrics
	elapsed := time.Since(startTime)
	finalProcessed := atomic.LoadInt64(&processedCount)
	finalFailed := atomic.LoadInt64(&failedCount)
	throughput := float64(finalProcessed) / elapsed.Seconds()

	// Print performance summary
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              RECALCULATION PERFORMANCE SUMMARY                ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-20s %38v ║\n", "Total Time:", elapsed.Round(time.Millisecond))
	fmt.Printf("║  %-20s %38d ║\n", "Total Processed:", finalProcessed)
	fmt.Printf("║  %-20s %38d ║\n", "Total Failed:", finalFailed)
	fmt.Printf("║  %-20s %34.0f /s ║\n", "Throughput:", throughput)
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")

	// Complete job
	if err := wp.jobRepo.Complete(ctx, jobID); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("Job %s completed successfully", jobID)
	return nil
}

// loadRoutingStepsCache loads all routing templates with their process steps into memory
func (wp *WorkerPool) loadRoutingStepsCache(ctx context.Context) (map[uuid.UUID][]*entity.ProcessStep, error) {
	cache := make(map[uuid.UUID][]*entity.ProcessStep)

	// Get all unique routing IDs from variants
	routingIDs, err := wp.variantRepo.ListUniqueRoutingIDs(ctx)
	if err != nil {
		return nil, err
	}

	// Load steps for each routing
	for _, routingID := range routingIDs {
		steps, err := wp.engine.processStepRepo.GetByRoutingID(ctx, routingID)
		if err != nil {
			log.Printf("Warning: failed to load steps for routing %s: %v", routingID, err)
			continue
		}
		cache[routingID] = steps
	}

	return cache, nil
}
