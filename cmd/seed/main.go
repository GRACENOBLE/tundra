package main

import (
	"fmt"
	"log"

	"github.com/GRACENOBLE/tundra/internal/database"
	"github.com/GRACENOBLE/tundra/internal/database/models"
	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Initialize database connection
	db := database.New().GetDB()

	// Hash the password
	password := "Hello@1234"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin user
	adminUser := models.User{
		Username: "admin",
		Email:    "admin@tundra.com",
		Password: string(hashedPassword),
		Role:     "admin",
	}

	// Check if admin user already exists
	var existingAdmin models.User
	result := db.Where("email = ?", adminUser.Email).First(&existingAdmin)
	if result.Error == nil {
		fmt.Println("⚠️  Admin user already exists, skipping creation")
	} else {
		if err := db.Create(&adminUser).Error; err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		fmt.Printf("✓ Created admin user: %s (%s)\n", adminUser.Email, adminUser.Username)
	}

	// Create regular user
	regularUser := models.User{
		Username: "user",
		Email:    "user@tundra.com",
		Password: string(hashedPassword),
		Role:     "user",
	}

	// Check if regular user already exists
	var existingUser models.User
	result = db.Where("email = ?", regularUser.Email).First(&existingUser)
	if result.Error == nil {
		fmt.Println("⚠️  Regular user already exists, skipping creation")
	} else {
		if err := db.Create(&regularUser).Error; err != nil {
			log.Fatalf("Failed to create regular user: %v", err)
		}
		fmt.Printf("✓ Created regular user: %s (%s)\n", regularUser.Email, regularUser.Username)
	}

	// Get or use admin user ID for product creation
	var adminForProducts models.User
	if err := db.Where("email = ?", adminUser.Email).First(&adminForProducts).Error; err != nil {
		log.Fatalf("Failed to get admin user: %v", err)
	}

	// Seed products
	fmt.Println("\nSeeding products...")
	products := []models.Product{
		{
			Name:        "Gaming Laptop Pro",
			Description: "High-performance gaming laptop with RTX 4080, 32GB RAM, and 1TB SSD",
			Price:       1999.99,
			Stock:       25,
			Category:    "Electronics",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "Wireless Mouse",
			Description: "Ergonomic wireless mouse with 6 programmable buttons",
			Price:       49.99,
			Stock:       150,
			Category:    "Accessories",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "Mechanical Keyboard",
			Description: "RGB mechanical keyboard with Cherry MX switches",
			Price:       129.99,
			Stock:       75,
			Category:    "Accessories",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "4K Monitor",
			Description: "27-inch 4K IPS monitor with 144Hz refresh rate",
			Price:       499.99,
			Stock:       40,
			Category:    "Electronics",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "USB-C Hub",
			Description: "7-in-1 USB-C hub with HDMI, USB 3.0, and card reader",
			Price:       39.99,
			Stock:       200,
			Category:    "Accessories",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "Wireless Headphones",
			Description: "Noise-cancelling Bluetooth headphones with 30-hour battery",
			Price:       199.99,
			Stock:       60,
			Category:    "Audio",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "Webcam HD",
			Description: "1080p webcam with auto-focus and dual microphones",
			Price:       79.99,
			Stock:       90,
			Category:    "Electronics",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "Laptop Stand",
			Description: "Aluminum laptop stand with adjustable height",
			Price:       34.99,
			Stock:       120,
			Category:    "Accessories",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "External SSD 1TB",
			Description: "Portable external SSD with 1050MB/s read speed",
			Price:       149.99,
			Stock:       80,
			Category:    "Storage",
			UserID:      adminForProducts.ID,
		},
		{
			Name:        "Cable Management Kit",
			Description: "Complete desk cable management solution with clips and sleeves",
			Price:       24.99,
			Stock:       180,
			Category:    "Accessories",
			UserID:      adminForProducts.ID,
		},
	}

	// Create products
	createdCount := 0
	skippedCount := 0
	for _, product := range products {
		var existing models.Product
		result := db.Where("name = ?", product.Name).First(&existing)
		if result.Error == nil {
			skippedCount++
			continue
		}

		if err := db.Create(&product).Error; err != nil {
			log.Printf("Failed to create product %s: %v", product.Name, err)
			continue
		}
		createdCount++
		fmt.Printf("✓ Created product: %s ($%.2f)\n", product.Name, product.Price)
	}

	if skippedCount > 0 {
		fmt.Printf("\n⚠️  Skipped %d existing products\n", skippedCount)
	}
	fmt.Printf("✓ Created %d new products\n", createdCount)

	fmt.Println("\n✅ Seeding completed successfully!")
	fmt.Println("\nYou can now login with:")
	fmt.Println("  Admin:    admin@tundra.com / Hello@1234")
	fmt.Println("  Regular:  user@tundra.com  / Hello@1234")
}
