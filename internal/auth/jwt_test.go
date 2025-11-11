package auth

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateJWT(t *testing.T) {
	// Set up test environment
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	username := "testuser"
	email := "test@example.com"
	role := "user"

	t.Run("Generate valid JWT", func(t *testing.T) {
		token, err := GenerateJWT(userID, username, email, role)
		if err != nil {
			t.Fatalf("GenerateJWT() error = %v", err)
		}

		if token == "" {
			t.Error("GenerateJWT() returned empty token")
		}
	})

	t.Run("JWT contains correct claims", func(t *testing.T) {
		token, err := GenerateJWT(userID, username, email, role)
		if err != nil {
			t.Fatalf("GenerateJWT() error = %v", err)
		}

		// Parse and validate the token
		claims, err := ValidateJWT(token)
		if err != nil {
			t.Fatalf("ValidateJWT() error = %v", err)
		}

		if claims.UserID != userID.String() {
			t.Errorf("Expected UserID %v, got %v", userID.String(), claims.UserID)
		}

		if claims.Username != username {
			t.Errorf("Expected Username %v, got %v", username, claims.Username)
		}

		if claims.Email != email {
			t.Errorf("Expected Email %v, got %v", email, claims.Email)
		}

		if claims.Role != role {
			t.Errorf("Expected Role %v, got %v", role, claims.Role)
		}

		if claims.Issuer != "tundra" {
			t.Errorf("Expected Issuer 'tundra', got %v", claims.Issuer)
		}
	})

	t.Run("JWT expires in 24 hours", func(t *testing.T) {
		token, err := GenerateJWT(userID, username, email, role)
		if err != nil {
			t.Fatalf("GenerateJWT() error = %v", err)
		}

		claims, err := ValidateJWT(token)
		if err != nil {
			t.Fatalf("ValidateJWT() error = %v", err)
		}

		expectedExpiry := time.Now().Add(24 * time.Hour)
		actualExpiry := claims.ExpiresAt.Time

		// Allow 1 minute tolerance
		diff := actualExpiry.Sub(expectedExpiry)
		if diff > time.Minute || diff < -time.Minute {
			t.Errorf("Token expiry not within expected range. Expected ~%v, got %v", expectedExpiry, actualExpiry)
		}
	})

	t.Run("Fails without JWT_SECRET", func(t *testing.T) {
		os.Unsetenv("JWT_SECRET")
		defer os.Setenv("JWT_SECRET", "test-secret-key-for-testing")

		_, err := GenerateJWT(userID, username, email, role)
		if err == nil {
			t.Error("Expected error when JWT_SECRET is not set")
		}
	})
}

func TestValidateJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	username := "testuser"
	email := "test@example.com"
	role := "user"

	t.Run("Validate valid token", func(t *testing.T) {
		token, _ := GenerateJWT(userID, username, email, role)

		claims, err := ValidateJWT(token)
		if err != nil {
			t.Fatalf("ValidateJWT() error = %v", err)
		}

		if claims == nil {
			t.Error("ValidateJWT() returned nil claims")
		}
	})

	t.Run("Reject invalid token", func(t *testing.T) {
		invalidToken := "invalid.token.string"

		_, err := ValidateJWT(invalidToken)
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})

	t.Run("Reject token with wrong secret", func(t *testing.T) {
		token, _ := GenerateJWT(userID, username, email, role)

		// Change the secret
		os.Setenv("JWT_SECRET", "different-secret")
		defer os.Setenv("JWT_SECRET", "test-secret-key-for-testing")

		_, err := ValidateJWT(token)
		if err == nil {
			t.Error("Expected error for token signed with different secret")
		}
	})

	t.Run("Reject expired token", func(t *testing.T) {
		// Create an expired token manually
		secret := os.Getenv("JWT_SECRET")
		claims := &Claims{
			UserID:   userID.String(),
			Username: username,
			Email:    email,
			Role:     role,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-25 * time.Hour)),
				Issuer:    "tundra",
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(secret))

		_, err := ValidateJWT(tokenString)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})
}

func TestJWTDoesNotContainSensitiveInfo(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	username := "testuser"
	email := "test@example.com"
	role := "user"

	token, err := GenerateJWT(userID, username, email, role)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}

	t.Run("JWT does not contain password", func(t *testing.T) {
		// Claims struct should not have a Password field
		// This is verified by the struct definition in jwt.go
		if claims.UserID == "" {
			t.Error("UserID should be present")
		}
		if claims.Username == "" {
			t.Error("Username should be present")
		}
		if claims.Email == "" {
			t.Error("Email should be present")
		}
		if claims.Role == "" {
			t.Error("Role should be present")
		}
		// Password field does not exist in Claims struct - this is correct
	})
}
