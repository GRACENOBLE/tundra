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

	fmt.Println("\n✅ Seeding completed successfully!")
	fmt.Println("\nYou can now login with:")
	fmt.Println("  Admin:    admin@tundra.com / Hello@1234")
	fmt.Println("  Regular:  user@tundra.com  / Hello@1234")
}
