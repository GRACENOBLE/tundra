package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestAuthMiddleware(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key")
	defer os.Unsetenv("JWT_SECRET")

	gin.SetMode(gin.TestMode)

	t.Run("Valid token passes authentication", func(t *testing.T) {
		userID := uuid.New()
		token, _ := GenerateJWT(userID, "testuser", "test@example.com", "user")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)

		AuthMiddleware()(c)

		// Check if middleware didn't abort (status would be set if aborted)
		if c.IsAborted() {
			t.Error("Expected middleware to not abort with valid token")
		}

		// Check if user info was set in context
		userIDFromContext, exists := c.Get("userID")
		if !exists {
			t.Error("Expected userID to be set in context")
		}
		if userIDFromContext != userID.String() {
			t.Errorf("Expected userID %s, got %s", userID.String(), userIDFromContext)
		}
	})

	t.Run("Missing Authorization header returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)

		AuthMiddleware()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Invalid token format returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "InvalidFormat")

		AuthMiddleware()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("Invalid token returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer invalid.token.here")

		AuthMiddleware()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})
}

func TestAdminMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Admin role passes authorization", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set("role", "admin")

		AdminMiddleware()(c)

		if c.IsAborted() {
			t.Error("Expected middleware to not abort for admin role")
		}
	})

	t.Run("User role returns 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set("role", "user")

		AdminMiddleware()(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("Missing role returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)

		AdminMiddleware()(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})
}
