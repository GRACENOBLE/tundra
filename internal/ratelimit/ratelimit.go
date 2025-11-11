package ratelimit

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimitConfig holds configuration for different rate limiters
type RateLimitConfig struct {
	// Global rate limit for all endpoints (e.g., "100-H" = 100 requests per hour)
	Global string
	// Auth rate limit for authentication endpoints (e.g., "5-M" = 5 requests per minute)
	Auth string
	// API rate limit for general API endpoints (e.g., "60-M" = 60 requests per minute)
	API string
}

// DefaultConfig returns sensible default rate limit configuration
func DefaultConfig() RateLimitConfig {
	return RateLimitConfig{
		Global: "1000-H", // 1000 requests per hour per IP
		Auth:   "5-M",    // 5 login/register attempts per minute per IP
		API:    "100-M",  // 100 API requests per minute per IP
	}
}

// NewRateLimiter creates a new rate limiter middleware with the specified rate
// Rate format: "limit-period" where period can be S (second), M (minute), H (hour)
// Examples: "10-S" (10/second), "100-M" (100/minute), "1000-H" (1000/hour)
func NewRateLimiter(rate string) gin.HandlerFunc {
	// Parse rate string
	rateLimit, err := limiter.NewRateFromFormatted(rate)
	if err != nil {
		// Fallback to a safe default if parsing fails
		rateLimit = limiter.Rate{
			Period: 1 * time.Minute,
			Limit:  60,
		}
	}

	// Create in-memory store for rate limiting
	store := memory.NewStore()

	// Create limiter instance
	instance := limiter.New(store, rateLimit)

	// Return Gin middleware
	return mgin.NewMiddleware(instance)
}

// GlobalLimiter creates a rate limiter for global API access
func GlobalLimiter() gin.HandlerFunc {
	return NewRateLimiter(DefaultConfig().Global)
}

// AuthLimiter creates a rate limiter for authentication endpoints
// More restrictive to prevent brute force attacks
func AuthLimiter() gin.HandlerFunc {
	return NewRateLimiter(DefaultConfig().Auth)
}

// APILimiter creates a rate limiter for general API endpoints
func APILimiter() gin.HandlerFunc {
	return NewRateLimiter(DefaultConfig().API)
}
