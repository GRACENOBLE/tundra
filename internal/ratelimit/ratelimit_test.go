package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		rate          string
		requestCount  int
		expectBlocked bool
		description   string
	}{
		{
			name:          "Within limit",
			rate:          "5-S",
			requestCount:  3,
			expectBlocked: false,
			description:   "Should allow requests within the limit",
		},
		{
			name:          "Exceeds limit",
			rate:          "3-S",
			requestCount:  5,
			expectBlocked: true,
			description:   "Should block requests exceeding the limit",
		},
		{
			name:          "Exactly at limit",
			rate:          "5-S",
			requestCount:  5,
			expectBlocked: false,
			description:   "Should allow requests exactly at the limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(NewRateLimiter(tt.rate))
			r.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			blocked := false
			for i := 0; i < tt.requestCount; i++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "127.0.0.1:1234"
				resp := httptest.NewRecorder()
				r.ServeHTTP(resp, req)

				if resp.Code == http.StatusTooManyRequests {
					blocked = true
					break
				}
			}

			assert.Equal(t, tt.expectBlocked, blocked, tt.description)
		})
	}
}

func TestGlobalLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(GlobalLimiter())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make a few requests - should all succeed with default 1000/hour limit
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code, "Global limiter should allow normal traffic")
	}
}

func TestAuthLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(AuthLimiter())
	r.POST("/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged in"})
	})

	// Auth limiter is 5 per minute, so 6th request should be blocked
	req := httptest.NewRequest("POST", "/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	// First 5 should succeed
	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code, "First 5 auth requests should succeed")
	}

	// 6th should be blocked
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code, "6th auth request should be blocked")
}

func TestAPILimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(APILimiter())
	r.GET("/api/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"products": []string{}})
	})

	// API limiter is 100 per minute, test first few requests
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/api/products", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusOK, resp.Code, "API requests within limit should succeed")
	}
}

func TestRateLimiterHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(NewRateLimiter("10-S"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	// Check that rate limit headers are set
	assert.NotEmpty(t, resp.Header().Get("X-RateLimit-Limit"), "Should set rate limit header")
	assert.NotEmpty(t, resp.Header().Get("X-RateLimit-Remaining"), "Should set remaining requests header")
}

func TestDifferentIPsNotAffected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(NewRateLimiter("2-S")) // Very low limit for testing
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// IP 1 - exhaust limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
	}

	// IP 2 - should still work
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:5678"
	resp2 := httptest.NewRecorder()
	r.ServeHTTP(resp2, req2)

	assert.Equal(t, http.StatusOK, resp2.Code, "Different IP should not be affected by other IP's rate limit")
}

func TestInvalidRateFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	// Invalid rate format should fall back to default (60/minute)
	r.Use(NewRateLimiter("invalid-format"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Should still work with fallback rate
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code, "Should work with fallback rate on invalid format")
}

func TestRateLimitReset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(NewRateLimiter("2-S")) // 2 requests per second
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	// First 2 should succeed
	for i := 0; i < 2; i++ {
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}

	// 3rd should be blocked
	resp1 := httptest.NewRecorder()
	r.ServeHTTP(resp1, req)
	assert.Equal(t, http.StatusTooManyRequests, resp1.Code)

	// Wait for rate limit to reset
	time.Sleep(1100 * time.Millisecond)

	// Should work again after reset
	resp2 := httptest.NewRecorder()
	r.ServeHTTP(resp2, req)
	assert.Equal(t, http.StatusOK, resp2.Code, "Rate limit should reset after period")
}
