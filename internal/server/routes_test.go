package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"tundra/internal/auth"
	"tundra/internal/database/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Test database container and connection
var (
	testDB        *gorm.DB
	testContainer *postgres.PostgresContainer
)

// setupTestDatabase creates a PostgreSQL container and returns a DB connection
func setupTestDatabase(t *testing.T) *gorm.DB {
	ctx := context.Background()

	// Create PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	db, err := gorm.Open(postgresdriver.Open(connStr), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.User{}, &models.Product{}, &models.Order{}, &models.OrderProduct{})
	require.NoError(t, err)

	// Store container for cleanup
	testContainer = postgresContainer

	return db
}

// cleanupTestDatabase closes the database and stops the container
func cleanupTestDatabase(t *testing.T) {
	if testContainer != nil {
		ctx := context.Background()
		err := testContainer.Terminate(ctx)
		if err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}
}

// setupTestServer creates a test server with a test database
func setupTestServer(t *testing.T) (*Server, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// Set up test JWT secret
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db := setupTestDatabase(t)

	server := &Server{
		db: db,
	}

	router := gin.New()
	router.Use(gin.Recovery())

	return server, router
}

// Helper function to create a test user in the database
func createTestUser(t *testing.T, db *gorm.DB, username, email, password string) *models.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &models.User{
		Username: username,
		Email:    email,
		Password: string(hashedPassword),
		Role:     "user",
	}

	err = db.Create(user).Error
	require.NoError(t, err)

	return user
}

// ==================== Auth Handler Tests ====================

func TestSignUpHandler_Success(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	reqBody := map[string]string{
		"username": "testuser123",
		"email":    "test@example.com",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "User registered successfully", response["message"])
	assert.NotNil(t, response["user"])

	// Verify user was created in database
	var user models.User
	err = server.db.Where("email = ?", "test@example.com").First(&user).Error
	require.NoError(t, err)
	assert.Equal(t, "testuser123", user.Username)
}

func TestSignUpHandler_InvalidUsername(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	tests := []struct {
		name     string
		username string
		wantErr  string
	}{
		{
			name:     "Username with spaces",
			username: "test user",
			wantErr:  "username must be alphanumeric",
		},
		{
			name:     "Username with special characters",
			username: "test@user",
			wantErr:  "username must be alphanumeric",
		},
		{
			name:     "Empty username",
			username: "",
			wantErr:  "All fields are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{
				"username": tt.username,
				"email":    "test@example.com",
				"password": "Password123!",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusBadRequest, resp.Code)

			var response map[string]string
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["error"], tt.wantErr)
		})
	}
}

func TestSignUpHandler_InvalidEmail(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	tests := []struct {
		name  string
		email string
	}{
		{"Missing @", "testexample.com"},
		{"Missing domain", "test@"},
		{"Missing local part", "@example.com"},
		{"Invalid format", "not-an-email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{
				"username": "testuser123",
				"email":    tt.email,
				"password": "Password123!",
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusBadRequest, resp.Code)

			var response map[string]string
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["error"], "email must be a valid email address")
		})
	}
}

func TestSignUpHandler_WeakPassword(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	tests := []struct {
		name     string
		password string
		wantErr  string
	}{
		{
			name:     "Too short",
			password: "Pass1!",
			wantErr:  "at least 8 characters",
		},
		{
			name:     "No uppercase",
			password: "password123!",
			wantErr:  "uppercase letter",
		},
		{
			name:     "No lowercase",
			password: "PASSWORD123!",
			wantErr:  "lowercase letter",
		},
		{
			name:     "No number",
			password: "Password!",
			wantErr:  "number",
		},
		{
			name:     "No special character",
			password: "Password123",
			wantErr:  "special character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{
				"username": "testuser123",
				"email":    "test@example.com",
				"password": tt.password,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusBadRequest, resp.Code)

			var response map[string]string
			err := json.Unmarshal(resp.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["error"], tt.wantErr)
		})
	}
}

func TestSignUpHandler_DuplicateUsername(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	// Create existing user
	createTestUser(t, server.db, "existinguser", "existing@example.com", "Password123!")

	reqBody := map[string]string{
		"username": "existinguser",
		"email":    "new@example.com",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)

	var response map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Username is already taken")
}

func TestSignUpHandler_DuplicateEmail(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	// Create existing user
	createTestUser(t, server.db, "existinguser", "existing@example.com", "Password123!")

	reqBody := map[string]string{
		"username": "newuser",
		"email":    "existing@example.com",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)

	var response map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Email is already registered")
}

func TestLoginHandler_Success(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/login", server.loginHandler)

	// Create test user
	createTestUser(t, server.db, "testuser", "test@example.com", "Password123!")

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response["token"])
	assert.NotNil(t, response["user"])

	// Verify JWT token is valid
	token, ok := response["token"].(string)
	require.True(t, ok)

	claims, err := auth.ValidateJWT(token)
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, "testuser", claims.Username)
}

func TestLoginHandler_InvalidEmail(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/login", server.loginHandler)

	reqBody := map[string]string{
		"email":    "not-an-email",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)

	var response map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Invalid email format")
}

func TestLoginHandler_UserNotFound(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/login", server.loginHandler)

	reqBody := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)

	var response map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid credentials", response["error"])
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/login", server.loginHandler)

	// Create test user
	createTestUser(t, server.db, "testuser", "test@example.com", "Password123!")

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "WrongPassword123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)

	var response map[string]string
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid credentials", response["error"])
}

func TestLoginHandler_MissingFields(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/login", server.loginHandler)

	tests := []struct {
		name    string
		reqBody map[string]string
	}{
		{
			name: "Missing email",
			reqBody: map[string]string{
				"password": "Password123!",
			},
		},
		{
			name: "Missing password",
			reqBody: map[string]string{
				"email": "test@example.com",
			},
		},
		{
			name:    "Empty body",
			reqBody: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tt.reqBody)

			req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			assert.Equal(t, http.StatusBadRequest, resp.Code)
		})
	}
}

func TestPasswordHashing(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/register", server.signUpHandler)

	password := "Password123!"

	reqBody := map[string]string{
		"username": "testuser",
		"email":    "test@example.com",
		"password": password,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusCreated, resp.Code)

	// Verify password is hashed in database
	var user models.User
	err := server.db.Where("email = ?", "test@example.com").First(&user).Error
	require.NoError(t, err)

	// Password should not be stored in plain text
	assert.NotEqual(t, password, user.Password)

	// Verify hashed password can be verified
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	assert.NoError(t, err)
}

func TestJWTTokenGeneration(t *testing.T) {
	server, router := setupTestServer(t)
	defer cleanupTestDatabase(t)

	router.POST("/auth/login", server.loginHandler)

	// Create test user
	user := createTestUser(t, server.db, "testuser", "test@example.com", "Password123!")

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Password123!",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	token, ok := response["token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, token)

	// Validate token
	claims, err := auth.ValidateJWT(token)
	require.NoError(t, err)

	// Verify claims
	assert.Equal(t, user.ID.String(), claims.UserID)
	assert.Equal(t, user.Username, claims.Username)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, user.Role, claims.Role)
}
