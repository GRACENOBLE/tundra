package main

import (
    "flag"
    "log"
    "os"

    "tundra/internal/database"
    "tundra/internal/database/models"
)

func main() {
    var action string
    flag.StringVar(&action, "action", "up", "Migration action: up, down, refresh")
    flag.Parse()

    // Initialize database connection
    db := database.New()
    defer db.Close()

    gormDB := db.GetDB()

    switch action {
    case "up":
        log.Println("Running migrations...")
        if err := database.AutoMigrate(gormDB); err != nil {
            log.Fatalf("Failed to run migrations: %v", err)
        }
        log.Println("Migrations completed successfully!")

    case "down":
        log.Println("Rolling back migrations...")
        if err := gormDB.Migrator().DropTable(
            &models.Order{},
            &models.Product{},
            &models.User{},
            "order_products", // Drop the many-to-many join table
        ); err != nil {
            log.Fatalf("Failed to rollback migrations: %v", err)
        }
        log.Println("Rollback completed successfully!")

    case "refresh":
        log.Println("Refreshing migrations (drop + migrate)...")
        
        // Drop tables
        if err := gormDB.Migrator().DropTable(
            &models.Order{},
            &models.Product{},
            &models.User{},
            "order_products",
        ); err != nil {
            log.Fatalf("Failed to drop tables: %v", err)
        }
        
        // Run migrations
        if err := database.AutoMigrate(gormDB); err != nil {
            log.Fatalf("Failed to run migrations: %v", err)
        }
        
        log.Println("Refresh completed successfully!")

    default:
        log.Fatalf("Unknown action: %s. Use 'up', 'down', or 'refresh'", action)
    }

    os.Exit(0)
}