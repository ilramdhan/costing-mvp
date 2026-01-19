package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/ilramdhan/costing-mvp/config"
	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/infrastructure/persistence"
	"github.com/ilramdhan/costing-mvp/internal/modules/costing"
	"github.com/ilramdhan/costing-mvp/pkg/database"
)

func main() {
	godotenv.Load()

	cfg := config.Load()
	ctx := context.Background()

	// Database connection
	pool, err := database.NewPool(ctx, &cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize repositories
	masterYarnRepo := persistence.NewMasterYarnRepository(pool)
	variantRepo := persistence.NewYarnVariantRepository(pool)
	processStepRepo := persistence.NewProcessStepRepository(pool)
	costRepo := persistence.NewVariantProcessCostRepository(pool)
	summaryRepo := persistence.NewVariantCostSummaryRepository(pool)
	jobRepo := persistence.NewBatchJobRepository(pool)

	// Initialize calculation engine and worker pool
	engine := costing.NewCalculationEngine(variantRepo, processStepRepo, costRepo, summaryRepo)
	workerPool := costing.NewWorkerPool(engine, variantRepo, summaryRepo, jobRepo, cfg.Worker.Count, cfg.Worker.BatchSize)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:               "Textile Costing API",
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           120 * time.Second,
		DisableStartupMessage: false,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// API v1 routes
	api := app.Group("/api/v1")

	// Master Yarn endpoints
	api.Get("/master-yarns", func(c *fiber.Ctx) error {
		limit := c.QueryInt("limit", 20)
		offset := c.QueryInt("offset", 0)
		yarns, err := masterYarnRepo.List(ctx, limit, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		count, _ := masterYarnRepo.Count(ctx)
		return c.JSON(fiber.Map{
			"data":   yarns,
			"total":  count,
			"limit":  limit,
			"offset": offset,
		})
	})

	api.Get("/master-yarns/:id", func(c *fiber.Ctx) error {
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		yarn, err := masterYarnRepo.GetByID(ctx, id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(yarn)
	})

	// Variant endpoints
	api.Get("/variants/count", func(c *fiber.Ctx) error {
		count, err := variantRepo.Count(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"count": count})
	})

	// Cost Summary endpoints
	api.Get("/cost-summaries", func(c *fiber.Ctx) error {
		limit := c.QueryInt("limit", 20)
		offset := c.QueryInt("offset", 0)
		summaries, err := summaryRepo.List(ctx, limit, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"data":   summaries,
			"limit":  limit,
			"offset": offset,
		})
	})

	api.Get("/cost-summaries/:id", func(c *fiber.Ctx) error {
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		summary, err := summaryRepo.GetByVariantID(ctx, id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(summary)
	})

	// Recalculation endpoints
	api.Post("/recalculate/all", func(c *fiber.Ctx) error {
		// Create job
		now := time.Now()
		job := &entity.BatchJob{
			ID:        uuid.New(),
			JobType:   entity.JobTypeRecalculateAll,
			Status:    entity.JobStatusPending,
			CreatedAt: now,
			StartedAt: &now,
		}
		if err := jobRepo.Create(ctx, job); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Base parameters for calculation (would come from price_rates table in production)
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

		// Start async recalculation
		go func() {
			if err := workerPool.RecalculateAll(context.Background(), job.ID, baseParams); err != nil {
				log.Printf("Recalculation failed: %v", err)
				jobRepo.Fail(context.Background(), job.ID, err.Error())
			}
		}()

		return c.Status(202).JSON(fiber.Map{
			"job_id":  job.ID,
			"message": "Recalculation started",
			"status":  job.Status,
		})
	})

	// Job status endpoints
	api.Get("/jobs", func(c *fiber.Ctx) error {
		jobs, err := jobRepo.ListRecent(ctx, 20)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": jobs})
	})

	api.Get("/jobs/:id", func(c *fiber.Ctx) error {
		id, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
		}
		job, err := jobRepo.GetByID(ctx, id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(fiber.Map{
			"job":      job,
			"progress": job.Progress(),
		})
	})

	// Stats endpoint
	api.Get("/stats", func(c *fiber.Ctx) error {
		masterCount, _ := masterYarnRepo.Count(ctx)
		variantCount, _ := variantRepo.Count(ctx)
		return c.JSON(fiber.Map{
			"master_yarns":  masterCount,
			"yarn_variants": variantCount,
			"timestamp":     time.Now().Format(time.RFC3339),
		})
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		app.Shutdown()
	}()

	// Start server
	log.Printf("Starting API server on :%s", cfg.App.Port)
	if err := app.Listen(":" + cfg.App.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
