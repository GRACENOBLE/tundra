package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/GRACENOBLE/tundra/internal/database"
)

func main() {
	var action string
	var steps int
	flag.StringVar(&action, "action", "up", "Migration action: up, down, status, force, version")
	flag.IntVar(&steps, "steps", 0, "Number of steps to migrate (use with up/down)")
	flag.Parse()

	// Get database connection string
	dbService := database.New()
	defer dbService.Close()

	// Build connection string
	host := os.Getenv("BLUEPRINT_DB_HOST")
	port := os.Getenv("BLUEPRINT_DB_PORT")
	user := os.Getenv("BLUEPRINT_DB_USERNAME")
	password := os.Getenv("BLUEPRINT_DB_PASSWORD")
	dbname := os.Getenv("BLUEPRINT_DB_DATABASE")
	schema := os.Getenv("BLUEPRINT_DB_SCHEMA")
	sslmode := os.Getenv("BLUEPRINT_DB_SSLMODE")

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&search_path=%s",
		user, password, host, port, dbname, sslmode, schema,
	)

	// Initialize migrate instance
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize migrate: %v", err)
	}
	defer m.Close()

	// Handle different actions
	switch action {
	case "create":
		if len(flag.Args()) == 0 {
			log.Fatal("Please provide a migration name: -action=create <migration_name>")
		}
		migrationName := flag.Arg(0)
		if err := createMigration(migrationName); err != nil {
			log.Fatalf("Failed to create migration: %v", err)
		}
		log.Printf("✓ Migration files created successfully!")
		return

	case "up":
		if steps > 0 {
			log.Printf("Migrating up %d steps...\n", steps)
			if err := m.Steps(steps); err != nil && err != migrate.ErrNoChange {
				log.Fatalf("Failed to migrate up: %v", err)
			}
		} else {
			log.Println("Running all pending migrations...")
			if err := m.Up(); err != nil && err != migrate.ErrNoChange {
				log.Fatalf("Failed to migrate up: %v", err)
			}
		}
		log.Println("✓ Migrations completed successfully!")

	case "down":
		if steps > 0 {
			log.Printf("Rolling back %d steps...\n", steps)
			if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
				log.Fatalf("Failed to rollback: %v", err)
			}
		} else {
			log.Println("⚠️  WARNING: Rolling back ALL migrations...")
			if err := m.Down(); err != nil && err != migrate.ErrNoChange {
				log.Fatalf("Failed to rollback: %v", err)
			}
		}
		log.Println("✓ Rollback completed successfully!")

	case "status", "version":
		version, dirty, err := m.Version()
		if err != nil && err != migrate.ErrNilVersion {
			log.Fatalf("Failed to get version: %v", err)
		}
		if err == migrate.ErrNilVersion {
			log.Println("No migrations have been applied yet")
		} else {
			status := "clean"
			if dirty {
				status = "dirty (migration failed)"
			}
			log.Printf("Current version: %d (%s)\n", version, status)
		}

	case "force":
		if len(flag.Args()) == 0 {
			log.Fatal("Please specify a version to force: -action=force <version>")
		}
		var version int
		if _, err := fmt.Sscanf(flag.Arg(0), "%d", &version); err != nil {
			log.Fatalf("Invalid version number: %v", err)
		}
		log.Printf("Forcing version to %d...\n", version)
		if err := m.Force(version); err != nil {
			log.Fatalf("Failed to force version: %v", err)
		}
		log.Println("✓ Version forced successfully!")

	case "drop":
		log.Println("⚠️  WARNING: This will drop all tables!")
		log.Println("Use -action=down to rollback migrations instead.")
		os.Exit(1)

	default:
		log.Fatalf("Unknown action: %s\nAvailable actions: up, down, status, version, force", action)
	}

	os.Exit(0)
}

func createMigration(name string) error {
	timestamp := fmt.Sprintf("%d", getCurrentTimestamp())
	upFile := fmt.Sprintf("migrations/%s_%s.up.sql", timestamp, name)
	downFile := fmt.Sprintf("migrations/%s_%s.down.sql", timestamp, name)

	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll("migrations", 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Template for up migration
	upTemplate := `-- Migration: %s
-- Created at: %s
-- Add your SQL statements here

`

	// Template for down migration
	downTemplate := `-- Rollback: %s
-- Created at: %s
-- Add your rollback SQL statements here

`

	// Write up migration file
	upContent := fmt.Sprintf(upTemplate, name, timestamp)
	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}

	// Write down migration file
	downContent := fmt.Sprintf(downTemplate, name, timestamp)
	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	log.Printf("Created migration files:\n")
	log.Printf("  - %s\n", upFile)
	log.Printf("  - %s\n", downFile)

	return nil
}

func getCurrentTimestamp() int64 {
	// Using format: YYYYMMDDHHMMSS (e.g., 20241111123045)
	// This matches the golang-migrate timestamp format
	now := time.Now()
	timestamp := now.Format("20060102150405")

	var result int64
	fmt.Sscanf(timestamp, "%d", &result)
	return result
}
