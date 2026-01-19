package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/ilramdhan/costing-mvp/config"
	"github.com/ilramdhan/costing-mvp/internal/domain/entity"
	"github.com/ilramdhan/costing-mvp/internal/infrastructure/persistence"
	"github.com/ilramdhan/costing-mvp/pkg/database"
)

var (
	masterCount   = flag.Int("masters", 1000, "Number of master yarns to generate")
	childrenCount = flag.Int("children", 100, "Number of children per master")
	batchSize     = flag.Int("batch", 5000, "Batch size for COPY operations")
	workerCount   = flag.Int("workers", 10, "Number of parallel workers")
)

func main() {
	flag.Parse()
	godotenv.Load()

	// Print header
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          TEXTILE COSTING ENGINE - DATA SEEDER                 ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	totalVariants := *masterCount * *childrenCount
	log.Printf("Configuration:")
	log.Printf("  Masters:       %d", *masterCount)
	log.Printf("  Children/Master: %d", *childrenCount)
	log.Printf("  Total Variants:  %d", totalVariants)
	log.Printf("  Batch Size:      %d", *batchSize)
	log.Printf("  Workers:         %d", *workerCount)
	log.Printf("  CPU Cores:       %d", runtime.NumCPU())
	fmt.Println()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, &cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	overallStart := time.Now()
	var metrics PerformanceMetrics

	// Phase 1: Master Data
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	phaseStart := time.Now()
	if err := seedMasterData(ctx, pool); err != nil {
		log.Fatalf("Failed to seed master data: %v", err)
	}
	metrics.MasterDataTime = time.Since(phaseStart)

	// Phase 1.5: Price Rates
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if err := seedPriceRates(ctx, pool); err != nil {
		log.Fatalf("Failed to seed price rates: %v", err)
	}

	// Phase 2: Routing Data
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	phaseStart = time.Now()
	routingID, err := seedRoutingData(ctx, pool)
	if err != nil {
		log.Fatalf("Failed to seed routing data: %v", err)
	}
	metrics.RoutingDataTime = time.Since(phaseStart)

	// Phase 3: Yarn Data
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	phaseStart = time.Now()
	if err := seedYarnData(ctx, pool, routingID); err != nil {
		log.Fatalf("Failed to seed yarn data: %v", err)
	}
	metrics.YarnDataTime = time.Since(phaseStart)

	metrics.TotalTime = time.Since(overallStart)
	metrics.TotalMasters = int64(*masterCount)
	metrics.TotalVariants = int64(totalVariants)

	// Print performance summary
	printPerformanceSummary(metrics)
}

// PerformanceMetrics holds timing and throughput data
type PerformanceMetrics struct {
	TotalMasters    int64
	TotalVariants   int64
	MasterDataTime  time.Duration
	RoutingDataTime time.Duration
	YarnDataTime    time.Duration
	TotalTime       time.Duration
}

func printPerformanceSummary(m PerformanceMetrics) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                  PERFORMANCE SUMMARY                          ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-20s %38v ║\n", "Total Time:", m.TotalTime.Round(time.Millisecond))
	fmt.Println("╠───────────────────────────────────────────────────────────────╣")
	fmt.Printf("║  %-20s %38v ║\n", "Master Data:", m.MasterDataTime.Round(time.Millisecond))
	fmt.Printf("║  %-20s %38v ║\n", "Routing Data:", m.RoutingDataTime.Round(time.Millisecond))
	fmt.Printf("║  %-20s %38v ║\n", "Yarn Data:", m.YarnDataTime.Round(time.Millisecond))
	fmt.Println("╠───────────────────────────────────────────────────────────────╣")
	fmt.Printf("║  %-20s %38s ║\n", "Total Masters:", formatNumber(m.TotalMasters))
	fmt.Printf("║  %-20s %38s ║\n", "Total Variants:", formatNumber(m.TotalVariants))
	fmt.Println("╠───────────────────────────────────────────────────────────────╣")

	// Throughput
	if m.YarnDataTime.Seconds() > 0 {
		mastersPerSec := float64(m.TotalMasters) / m.YarnDataTime.Seconds()
		variantsPerSec := float64(m.TotalVariants) / m.YarnDataTime.Seconds()
		fmt.Printf("║  %-20s %34.0f /s ║\n", "Master Throughput:", mastersPerSec)
		fmt.Printf("║  %-20s %34.0f /s ║\n", "Variant Throughput:", variantsPerSec)
	}

	fmt.Println("╠───────────────────────────────────────────────────────────────╣")
	fmt.Printf("║  %-20s %35s MB ║\n", "Memory Allocated:", formatNumber(int64(memStats.Alloc/1024/1024)))
	fmt.Printf("║  %-20s %35s MB ║\n", "Total Allocated:", formatNumber(int64(memStats.TotalAlloc/1024/1024)))
	fmt.Printf("║  %-20s %35s MB ║\n", "Sys Memory:", formatNumber(int64(memStats.Sys/1024/1024)))
	fmt.Printf("║  %-20s %38d ║\n", "GC Cycles:", memStats.NumGC)
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
}

func formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	var result []rune
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, r)
	}
	return string(result)
}

func seedMasterData(ctx context.Context, pool *pgxpool.Pool) error {
	log.Println("Seeding parameter groups and master parameters...")

	// Parameter groups
	groups := []string{
		"raw_material", "electricity", "labor", "machine", "quality", "packaging", "overhead",
	}

	for _, g := range groups {
		_, err := pool.Exec(ctx, `
			INSERT INTO parameter_groups (code, name) VALUES ($1, $2)
			ON CONFLICT (code) DO NOTHING
		`, g, g+" Parameters")
		if err != nil {
			return fmt.Errorf("failed to insert parameter group %s: %w", g, err)
		}
	}

	// Master parameters (250 parameters)
	parameterNames := generateParameterNames(250)
	for i, name := range parameterNames {
		groupCode := groups[i%len(groups)]
		_, err := pool.Exec(ctx, `
			INSERT INTO master_parameters (key, label, data_type, default_value, group_code, sequence_order)
			VALUES ($1, $2, 'float', '0', $3, $4)
			ON CONFLICT (key) DO NOTHING
		`, name, name, groupCode, i)
		if err != nil {
			return fmt.Errorf("failed to insert parameter %s: %w", name, err)
		}
	}

	log.Printf("Created %d parameter groups and %d master parameters", len(groups), len(parameterNames))
	return nil
}

func seedPriceRates(ctx context.Context, pool *pgxpool.Pool) error {
	log.Println("Seeding price rates...")

	// Sample price rates for common parameters
	priceRates := map[string]float64{
		"raw_material":     50.0,
		"electricity_kwh":  1.5,
		"labor_hours":      25.0,
		"machine_hours":    15.0,
		"water_liters":     0.02,
		"steam_hours":      10.0,
		"chemical_kg":      80.0,
		"dye_kg":           100.0,
		"spindle_hours":    15.0,
		"loom_hours":       20.0,
		"finishing_hours":  12.0,
		"packaging_units":  5.0,
		"waste_percentage": 0.05,
		"quality_factor":   1.0,
		"efficiency_rate":  0.95,
		"overhead_rate":    0.1,
		"input_cost":       1000.0,
		"output_cost":      1200.0,
		"material_price":   50.0,
		"labor_rate":       25.0,
	}

	effectiveDate := time.Now().Format("2006-01-02")
	count := 0

	for paramKey, rateValue := range priceRates {
		_, err := pool.Exec(ctx, `
			INSERT INTO price_rates (id, parameter_key, rate_value, effective_date, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (parameter_key, effective_date) DO UPDATE SET rate_value = EXCLUDED.rate_value
		`, uuid.New(), paramKey, rateValue, effectiveDate, "Monthly rate")
		if err != nil {
			// Skip if parameter_key doesn't exist (foreign key constraint)
			continue
		}
		count++
	}

	log.Printf("Created %d price rates", count)
	return nil
}

func seedRoutingData(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	log.Println("Seeding process masters and routing templates...")

	// Process masters
	processes := []struct {
		code     string
		name     string
		sequence int
	}{
		{"SMELTING", "Smelting Process", 1},
		{"SPINNING", "Spinning Process", 2},
		{"WEAVING", "Weaving Process", 3},
		{"DYEING", "Dyeing Process", 4},
		{"FINISHING", "Finishing Process", 5},
		{"PACKING", "Packing Process", 6},
	}

	processIDs := make([]uuid.UUID, len(processes))
	for i, p := range processes {
		id := uuid.New()
		processIDs[i] = id
		_, err := pool.Exec(ctx, `
			INSERT INTO process_masters (id, code, name, default_sequence, created_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (code) DO UPDATE SET id = EXCLUDED.id RETURNING id
		`, id, p.code, p.name, p.sequence)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert process %s: %w", p.code, err)
		}
	}

	// Routing template
	routingID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO routing_templates (id, name, description, is_active, created_at)
		VALUES ($1, 'Standard Textile Route', 'Full textile production route', true, NOW())
		ON CONFLICT (name) DO UPDATE SET id = EXCLUDED.id RETURNING id
	`, routingID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert routing template: %w", err)
	}

	// Process steps with formulas
	formulas := []string{
		"(raw_material_kg * material_price) + (electricity_kwh_1 * electricity_rate) + (labor_hours_1 * labor_rate)",
		"(input_cost_1 * 1.0) + (spindle_hours * spindle_rate) + (labor_hours_2 * labor_rate)",
		"(input_cost_2 * 1.0) + (loom_hours * loom_rate) + (labor_hours_3 * labor_rate)",
		"(input_cost_3 * 1.0) + (dye_kg * dye_price) + (water_liters * water_rate) + (steam_hours * steam_rate)",
		"(input_cost_4 * 1.0) + (finishing_hours * finishing_rate) + (chemical_kg * chemical_price)",
		"(input_cost_5 * 1.0) + (packaging_units * packaging_price) + (labor_hours_6 * labor_rate)",
	}

	for i, processID := range processIDs {
		stepID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO process_steps (id, routing_template_id, process_master_id, sequence_order, formula_expression, created_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (routing_template_id, sequence_order) DO NOTHING
		`, stepID, routingID, processID, i+1, formulas[i])
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to insert process step %d: %w", i+1, err)
		}
	}

	log.Printf("Created %d process masters, 1 routing template, and %d process steps", len(processes), len(processes))
	return routingID, nil
}

func seedYarnData(ctx context.Context, pool *pgxpool.Pool, routingID uuid.UUID) error {
	log.Println("Seeding master yarns and variants...")

	masterRepo := persistence.NewMasterYarnRepository(pool)
	variantRepo := persistence.NewYarnVariantRepository(pool)

	totalVariants := *masterCount * *childrenCount
	log.Printf("Will create %d master yarns and %d total variants", *masterCount, totalVariants)

	// Use worker pool for parallel seeding
	numWorkers := *workerCount
	masterChan := make(chan int, numWorkers*2)

	var (
		completedMasters  int64
		completedVariants int64
		wg                sync.WaitGroup
	)

	// Progress reporter
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m := atomic.LoadInt64(&completedMasters)
			v := atomic.LoadInt64(&completedVariants)
			if m >= int64(*masterCount) {
				return
			}
			log.Printf("Progress: masters=%d/%d (%.1f%%), variants=%d/%d (%.1f%%)",
				m, *masterCount, float64(m)/float64(*masterCount)*100,
				v, totalVariants, float64(v)/float64(totalVariants)*100)
		}
	}()

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			masterBatch := make([]*entity.MasterYarn, 0, *batchSize / *childrenCount)
			variantBatch := make([]*entity.YarnVariant, 0, *batchSize)

			for masterIdx := range masterChan {
				now := time.Now()
				masterID := uuid.New()

				// Create master yarn with fixed attrs
				fixedAttrs := generateFixedAttrs()
				master := &entity.MasterYarn{
					ID:         masterID,
					Code:       fmt.Sprintf("YARN-%06d", masterIdx),
					Name:       fmt.Sprintf("Master Yarn %d", masterIdx),
					FixedAttrs: fixedAttrs,
					IsActive:   true,
					CreatedAt:  now,
					UpdatedAt:  now,
				}
				masterBatch = append(masterBatch, master)

				// Create variants for this master
				for j := 0; j < *childrenCount; j++ {
					variant := &entity.YarnVariant{
						ID:                uuid.New(),
						MasterYarnID:      masterID,
						SKU:               fmt.Sprintf("SKU-%06d-%04d", masterIdx, j),
						BatchNo:           fmt.Sprintf("BATCH-%d", j%100),
						RoutingTemplateID: routingID,
						IsActive:          true,
						CreatedAt:         now,
						UpdatedAt:         now,
					}
					variantBatch = append(variantBatch, variant)
				}

				// Flush batches when full
				if len(variantBatch) >= *batchSize {
					// Insert masters first
					if len(masterBatch) > 0 {
						if _, err := masterRepo.CreateBatch(ctx, masterBatch); err != nil {
							log.Printf("Worker %d: failed to insert masters: %v", workerID, err)
						}
						atomic.AddInt64(&completedMasters, int64(len(masterBatch)))
						masterBatch = masterBatch[:0]
					}

					// Insert variants
					if _, err := variantRepo.CreateBatch(ctx, variantBatch); err != nil {
						log.Printf("Worker %d: failed to insert variants: %v", workerID, err)
					}
					atomic.AddInt64(&completedVariants, int64(len(variantBatch)))
					variantBatch = variantBatch[:0]
				}
			}

			// Flush remaining
			if len(masterBatch) > 0 {
				if _, err := masterRepo.CreateBatch(ctx, masterBatch); err != nil {
					log.Printf("Worker %d: failed to insert remaining masters: %v", workerID, err)
				}
				atomic.AddInt64(&completedMasters, int64(len(masterBatch)))
			}
			if len(variantBatch) > 0 {
				if _, err := variantRepo.CreateBatch(ctx, variantBatch); err != nil {
					log.Printf("Worker %d: failed to insert remaining variants: %v", workerID, err)
				}
				atomic.AddInt64(&completedVariants, int64(len(variantBatch)))
			}
		}(w)
	}

	// Send work to workers
	for i := 0; i < *masterCount; i++ {
		masterChan <- i
	}
	close(masterChan)

	wg.Wait()

	log.Printf("Completed: %d masters and %d variants created",
		atomic.LoadInt64(&completedMasters), atomic.LoadInt64(&completedVariants))
	return nil
}

func generateParameterNames(count int) []string {
	prefixes := []string{
		"raw_material", "electricity_kwh", "labor_hours", "machine_hours",
		"water_liters", "steam_hours", "chemical_kg", "dye_kg",
		"spindle_hours", "loom_hours", "finishing_hours", "packaging_units",
		"waste_percentage", "quality_factor", "efficiency_rate", "overhead_rate",
		"input_cost", "output_cost", "material_price", "labor_rate",
	}

	names := make([]string, count)
	for i := 0; i < count; i++ {
		prefix := prefixes[i%len(prefixes)]
		suffix := i / len(prefixes)
		if suffix > 0 {
			names[i] = fmt.Sprintf("%s_%d", prefix, suffix)
		} else {
			names[i] = prefix
		}
	}
	return names
}

func generateFixedAttrs() map[string]interface{} {
	return map[string]interface{}{
		"fiber_type":     randomChoice([]string{"cotton", "polyester", "wool", "silk", "blend"}),
		"yarn_count":     rand.Intn(100) + 10,
		"twist_per_inch": rand.Float64()*20 + 5,
		"strength_gf":    rand.Float64()*500 + 100,
		"elongation_pct": rand.Float64()*15 + 5,
		"moisture_pct":   rand.Float64()*3 + 5,
		"grade":          randomChoice([]string{"A", "B", "C", "Premium"}),
		"color_code":     fmt.Sprintf("#%06x", rand.Intn(0xFFFFFF)),
		"weight_grams":   rand.Float64()*100 + 50,
		"diameter_mm":    rand.Float64()*2 + 0.5,
	}
}

func randomChoice(choices []string) string {
	return choices[rand.Intn(len(choices))]
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())
}
