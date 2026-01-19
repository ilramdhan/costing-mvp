package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/ilramdhan/costing-mvp/config"
	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/domain/repository"
	"github.com/ilramdhan/costing-mvp/internal/infrastructure/persistence"
	"github.com/ilramdhan/costing-mvp/internal/modules/costing"
	"github.com/ilramdhan/costing-mvp/pkg/database"
)

func main() {
	godotenv.Load()

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting worker service with %d workers and batch size %d",
		cfg.Worker.Count, cfg.Worker.BatchSize)

	// Database connection
	pool, err := database.NewPool(ctx, &cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize repositories
	variantRepo := persistence.NewYarnVariantRepository(pool)
	processStepRepo := persistence.NewProcessStepRepository(pool)
	costRepo := persistence.NewVariantProcessCostRepository(pool)
	summaryRepo := persistence.NewVariantCostSummaryRepository(pool)
	jobRepo := persistence.NewBatchJobRepository(pool)

	// Initialize calculation engine and worker pool
	engine := costing.NewCalculationEngine(variantRepo, processStepRepo, costRepo, summaryRepo)
	workerPool := costing.NewWorkerPool(engine, variantRepo, summaryRepo, jobRepo, cfg.Worker.Count, cfg.Worker.BatchSize)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Worker mode: process pending jobs or wait for manual trigger
	log.Println("Worker service ready. Waiting for jobs...")

	// Check for pending jobs periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			log.Println("Shutting down worker service...")
			cancel()
			return

		case <-ticker.C:
			// Check for pending jobs
			jobs, err := jobRepo.ListRecent(ctx, 10)
			if err != nil {
				log.Printf("Failed to list jobs: %v", err)
				continue
			}

			for _, job := range jobs {
				if job.Status == entity.JobStatusPending {
					log.Printf("Found pending job: %s", job.ID)
					processJob(ctx, workerPool, jobRepo, job)
				}
			}
		}
	}
}

func processJob(ctx context.Context, workerPool *costing.WorkerPool, jobRepo repository.BatchJobRepository, job *entity.BatchJob) {
	// Base parameters (in production, fetch from price_rates table)
	baseParams := map[string]interface{}{
		"material_price":      50.0,
		"electricity_rate":    1.5,
		"labor_rate":          25.0,
		"spindle_rate":        15.0,
		"loom_rate":           20.0,
		"dye_price":           100.0,
		"water_rate":          0.02,
		"steam_rate":          10.0,
		"finishing_rate":      12.0,
		"chemical_price":      80.0,
		"packaging_price":     5.0,
		"overhead_percentage": 0.1,
		"raw_material_kg":     100.0,
		"electricity_kwh_1":   50.0,
		"labor_hours_1":       8.0,
		"input_cost_1":        5000.0,
		"spindle_hours":       10.0,
		"labor_hours_2":       6.0,
		"input_cost_2":        6000.0,
		"loom_hours":          8.0,
		"labor_hours_3":       5.0,
		"input_cost_3":        7000.0,
		"dye_kg":              2.5,
		"water_liters":        500.0,
		"steam_hours":         5.0,
		"input_cost_4":        8000.0,
		"finishing_hours":     4.0,
		"chemical_kg":         1.5,
		"input_cost_5":        9000.0,
		"packaging_units":     10.0,
		"labor_hours_6":       3.0,
		"material_cost":       1000.0,
	}

	startTime := time.Now()
	log.Printf("Starting job %s at %s", job.ID, startTime.Format(time.RFC3339))

	if err := workerPool.RecalculateAll(ctx, job.ID, baseParams); err != nil {
		log.Printf("Job %s failed: %v", job.ID, err)
		return
	}

	elapsed := time.Since(startTime)
	log.Printf("Job %s completed in %v", job.ID, elapsed)
}
