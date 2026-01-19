package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/ilramdhan/costing-mvp/config"
	"github.com/ilramdhan/costing-mvp/pkg/database"
)

func main() {
	godotenv.Load()

	upCmd := flag.NewFlagSet("up", flag.ExitOnError)
	downCmd := flag.NewFlagSet("down", flag.ExitOnError)
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)

	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <command>")
		fmt.Println("Commands: up, down, status")
		os.Exit(1)
	}

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, &cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Ensure migrations table exists
	ensureMigrationsTable(ctx, pool)

	switch os.Args[1] {
	case "up":
		upCmd.Parse(os.Args[2:])
		runMigrationsUp(ctx, pool)
	case "down":
		downCmd.Parse(os.Args[2:])
		runMigrationsDown(ctx, pool)
	case "status":
		statusCmd.Parse(os.Args[2:])
		showMigrationStatus(ctx, pool)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}
}

func runMigrationsUp(ctx context.Context, pool *pgxpool.Pool) {
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		log.Fatalf("Failed to find migration files: %v", err)
	}
	sort.Strings(files)

	for _, file := range files {
		version := extractVersion(file)
		if isApplied(ctx, pool, version) {
			log.Printf("Skipping %s (already applied)", version)
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", file, err)
		}

		log.Printf("Applying %s...", version)
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			log.Fatalf("Failed to apply %s: %v", file, err)
		}

		if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			log.Fatalf("Failed to record migration %s: %v", version, err)
		}
		log.Printf("Applied %s successfully", version)
	}
}

func runMigrationsDown(ctx context.Context, pool *pgxpool.Pool) {
	files, err := filepath.Glob("migrations/*.down.sql")
	if err != nil {
		log.Fatalf("Failed to find migration files: %v", err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	// Only rollback the latest migration
	if len(files) == 0 {
		log.Println("No migrations to rollback")
		return
	}

	file := files[0]
	version := extractVersion(file)
	if !isApplied(ctx, pool, version) {
		log.Printf("Migration %s is not applied", version)
		return
	}

	content, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", file, err)
	}

	log.Printf("Rolling back %s...", version)
	if _, err := pool.Exec(ctx, string(content)); err != nil {
		log.Fatalf("Failed to rollback %s: %v", file, err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		log.Fatalf("Failed to remove migration record %s: %v", version, err)
	}
	log.Printf("Rolled back %s successfully", version)
}

func showMigrationStatus(ctx context.Context, pool *pgxpool.Pool) {
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		log.Fatalf("Failed to find migration files: %v", err)
	}
	sort.Strings(files)

	fmt.Println("Migration Status:")
	fmt.Println("=================")
	for _, file := range files {
		version := extractVersion(file)
		status := "PENDING"
		if isApplied(ctx, pool, version) {
			status = "APPLIED"
		}
		fmt.Printf("[%s] %s\n", status, version)
	}
}

func extractVersion(filename string) string {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return base
}

func isApplied(ctx context.Context, pool *pgxpool.Pool, version string) bool {
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}
