package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/GRACENOBLE/tundra/internal/cloudinary"
	"github.com/GRACENOBLE/tundra/internal/database"
)

type Server struct {
	port int

	db         *gorm.DB
	redis      *redis.Client
	cloudinary *cloudinary.Client
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"), // empty if no password
		DB:       0,                           // use default DB
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("Warning: Redis connection failed: %v. Caching will be disabled.\n", err)
		redisClient = nil
	}

	// Initialize Cloudinary client
	cloudinaryClient, err := cloudinary.NewClient()
	if err != nil {
		fmt.Printf("Warning: Cloudinary initialization failed: %v. Image uploads will be disabled.\n", err)
		cloudinaryClient = nil
	}

	NewServer := &Server{
		port:       port,
		db:         database.New().GetDB(),
		redis:      redisClient,
		cloudinary: cloudinaryClient,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
