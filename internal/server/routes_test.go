package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"tundra/internal/auth"
	"tundra/internal/database/models"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	redisContainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Test database container and connection
var (
	testContainer      *postgres.PostgresContainer
	testRedisContainer *redisContainer.RedisContainer
)

// setupTestRedis creates a Redis container and returns a Redis client
func setupTestRedis(t *testing.T) *redis.Client {
	ctx := context.Background()

	// Create Redis container
	container, err := redisContainer.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	// Create Redis client
	opt, err := redis.ParseURL(connStr)
	require.NoError(t, err)

	client := redis.NewClient(opt)

	// Test connection
	err = client.Ping(ctx).Err()
	require.NoError(t, err)

	// Store container for cleanup
	testRedisContainer = container

	return client
}

// cleanupTestRedis stops the Redis container
func cleanupTestRedis(t *testing.T) {
	if testRedisContainer != nil {
		ctx := context.Background()
		err := testRedisContainer.Terminate(ctx)
		if err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}
}

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
		db:    db,
		redis: nil, // No Redis by default for auth tests
	}

	router := gin.New()
	router.Use(gin.Recovery())

	return server, router
}

// setupTestServerWithRedis creates a test server with both database and Redis
func setupTestServerWithRedis(t *testing.T) (*Server, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	// Set up test JWT secret
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db := setupTestDatabase(t)
	redisClient := setupTestRedis(t)

	server := &Server{
		db:    db,
		redis: redisClient,
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

// Helper function to create multipart form data for product creation
func createProductFormData(product map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add form fields
	for key, value := range product {
		_ = writer.WriteField(key, value)
	}

	writer.Close()
	return body, writer.FormDataContentType()
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

// ==================== Redis Caching Tests ====================

func TestProductListCaching_CacheHit(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)

	// Create test products
	products := []models.Product{
		{Name: "Product 1", Description: "Desc 1", Price: 10.0, Stock: 5, Category: "Cat1"},
		{Name: "Product 2", Description: "Desc 2", Price: 20.0, Stock: 10, Category: "Cat2"},
	}
	for _, p := range products {
		err := server.db.Create(&p).Error
		require.NoError(t, err)
	}

	// First request - should populate cache
	req1, _ := http.NewRequest("GET", "/products", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	var response1 map[string]interface{}
	err := json.Unmarshal(resp1.Body.Bytes(), &response1)
	require.NoError(t, err)

	// Second request - should hit cache
	req2, _ := http.NewRequest("GET", "/products", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	var response2 map[string]interface{}
	err = json.Unmarshal(resp2.Body.Bytes(), &response2)
	require.NoError(t, err)

	// Responses should be identical
	assert.Equal(t, response1["totalProducts"], response2["totalProducts"])
	assert.Equal(t, response1["currentPage"], response2["currentPage"])
}

func TestProductListCaching_DifferentPages(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)

	// Create 15 test products
	for i := 1; i <= 15; i++ {
		product := models.Product{
			Name:        fmt.Sprintf("Product %d", i),
			Description: fmt.Sprintf("Description %d", i),
			Price:       float64(i * 10),
			Stock:       int64(i * 5),
			Category:    "Test",
		}
		err := server.db.Create(&product).Error
		require.NoError(t, err)
	}

	// Request page 1
	req1, _ := http.NewRequest("GET", "/products?page=1&pageSize=10", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	// Request page 2
	req2, _ := http.NewRequest("GET", "/products?page=2&pageSize=10", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	// Both should be cached separately
	var response1, response2 map[string]interface{}
	json.Unmarshal(resp1.Body.Bytes(), &response1)
	json.Unmarshal(resp2.Body.Bytes(), &response2)

	assert.Equal(t, float64(1), response1["currentPage"])
	assert.Equal(t, float64(2), response2["currentPage"])
	assert.Equal(t, float64(10), response1["pageSize"])
	assert.Equal(t, float64(5), response2["pageSize"])
}

func TestProductListCaching_SearchQueries(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)

	// Create test products
	products := []models.Product{
		{Name: "Apple iPhone", Description: "Smartphone", Price: 999.0, Stock: 10, Category: "Electronics"},
		{Name: "Apple Watch", Description: "Smartwatch", Price: 399.0, Stock: 5, Category: "Electronics"},
		{Name: "Samsung Galaxy", Description: "Smartphone", Price: 899.0, Stock: 8, Category: "Electronics"},
	}
	for _, p := range products {
		err := server.db.Create(&p).Error
		require.NoError(t, err)
	}

	// Search for "Apple"
	req1, _ := http.NewRequest("GET", "/products?search=Apple", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	var response1 map[string]interface{}
	json.Unmarshal(resp1.Body.Bytes(), &response1)
	assert.Equal(t, float64(2), response1["totalProducts"]) // 2 Apple products

	// Search for "Samsung"
	req2, _ := http.NewRequest("GET", "/products?search=Samsung", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	var response2 map[string]interface{}
	json.Unmarshal(resp2.Body.Bytes(), &response2)
	assert.Equal(t, float64(1), response2["totalProducts"]) // 1 Samsung product

	// Different search queries should have separate cache entries
	assert.NotEqual(t, response1["totalProducts"], response2["totalProducts"])
}

func TestProductListCaching_InvalidationOnCreate(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)
	router.POST("/products", auth.AuthMiddleware(), auth.AdminMiddleware(), server.createProductHandler)

	// Create admin user
	adminUser := createTestUser(t, server.db, "admin", "admin@test.com", "Password123!")
	adminUser.Role = "admin"
	server.db.Save(adminUser)

	// Get admin token
	token, err := auth.GenerateJWT(adminUser.ID, adminUser.Username, adminUser.Email, adminUser.Role)
	require.NoError(t, err)

	// First request - populate cache
	req1, _ := http.NewRequest("GET", "/products", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	var response1 map[string]interface{}
	json.Unmarshal(resp1.Body.Bytes(), &response1)
	initialCount := response1["totalProducts"]

	// Create a new product (should invalidate cache)
	newProduct := map[string]string{
		"name":        "New Product",
		"description": "New Description",
		"price":       "50.0",
		"stock":       "10",
		"category":    "New Category",
	}
	productBody, contentType := createProductFormData(newProduct)

	createReq, _ := http.NewRequest("POST", "/products", productBody)
	createReq.Header.Set("Content-Type", contentType)
	createReq.Header.Set("Authorization", "Bearer "+token)
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	require.Equal(t, http.StatusCreated, createResp.Code)

	// Request products again - should get fresh data from DB
	req2, _ := http.NewRequest("GET", "/products", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	var response2 map[string]interface{}
	json.Unmarshal(resp2.Body.Bytes(), &response2)
	newCount := response2["totalProducts"]

	// Count should have increased
	assert.Equal(t, initialCount.(float64)+1, newCount.(float64))
}

func TestProductListCaching_InvalidationOnUpdate(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)
	router.PUT("/products/:id", auth.AuthMiddleware(), auth.AdminMiddleware(), server.updateProductHandler)

	// Create admin user
	adminUser := createTestUser(t, server.db, "admin", "admin@test.com", "Password123!")
	adminUser.Role = "admin"
	server.db.Save(adminUser)

	// Create a product
	product := models.Product{
		Name:        "Original Product",
		Description: "Original Description",
		Price:       100.0,
		Stock:       10,
		Category:    "Original",
	}
	err := server.db.Create(&product).Error
	require.NoError(t, err)

	// Get admin token
	token, err := auth.GenerateJWT(adminUser.ID, adminUser.Username, adminUser.Email, adminUser.Role)
	require.NoError(t, err)

	// First request - populate cache
	req1, _ := http.NewRequest("GET", "/products", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	// Update the product (should invalidate cache)
	updateData := map[string]interface{}{
		"name":  "Updated Product",
		"price": 150.0,
	}
	updateJSON, _ := json.Marshal(updateData)

	updateReq, _ := http.NewRequest("PUT", "/products/"+product.ID.String(), bytes.NewBuffer(updateJSON))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	require.Equal(t, http.StatusOK, updateResp.Code)

	// Request products again - should get fresh data
	req2, _ := http.NewRequest("GET", "/products", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	var response2 map[string]interface{}
	json.Unmarshal(resp2.Body.Bytes(), &response2)

	// Verify we got the updated product
	productsArray := response2["products"].([]interface{})
	assert.Greater(t, len(productsArray), 0)
}

func TestProductListCaching_InvalidationOnDelete(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)
	router.DELETE("/products/:id", auth.AuthMiddleware(), auth.AdminMiddleware(), server.deleteProductHandler)

	// Create admin user
	adminUser := createTestUser(t, server.db, "admin", "admin@test.com", "Password123!")
	adminUser.Role = "admin"
	server.db.Save(adminUser)

	// Create products
	product1 := models.Product{Name: "Product 1", Description: "Desc", Price: 10.0, Stock: 5, Category: "Cat"}
	product2 := models.Product{Name: "Product 2", Description: "Desc", Price: 20.0, Stock: 10, Category: "Cat"}
	server.db.Create(&product1)
	server.db.Create(&product2)

	// Get admin token
	token, err := auth.GenerateJWT(adminUser.ID, adminUser.Username, adminUser.Email, adminUser.Role)
	require.NoError(t, err)

	// First request - populate cache
	req1, _ := http.NewRequest("GET", "/products", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	var response1 map[string]interface{}
	json.Unmarshal(resp1.Body.Bytes(), &response1)
	initialCount := response1["totalProducts"].(float64)

	// Delete a product (should invalidate cache)
	deleteReq, _ := http.NewRequest("DELETE", "/products/"+product1.ID.String(), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	// Request products again - should get fresh data
	req2, _ := http.NewRequest("GET", "/products", nil)
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	require.Equal(t, http.StatusOK, resp2.Code)

	var response2 map[string]interface{}
	json.Unmarshal(resp2.Body.Bytes(), &response2)
	newCount := response2["totalProducts"].(float64)

	// Count should have decreased
	assert.Equal(t, initialCount-1, newCount)
}

func TestProductListCaching_WorksWithoutRedis(t *testing.T) {
	server, router := setupTestServer(t) // No Redis
	defer cleanupTestDatabase(t)

	router.GET("/products", server.listProductsHandler)

	// Create test product
	product := models.Product{
		Name:        "Test Product",
		Description: "Description",
		Price:       10.0,
		Stock:       5,
		Category:    "Category",
	}
	err := server.db.Create(&product).Error
	require.NoError(t, err)

	// Request should work even without Redis
	req, _ := http.NewRequest("GET", "/products", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	err = json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(1), response["totalProducts"])
}

func TestProductListCaching_CacheExpiration(t *testing.T) {
	server, router := setupTestServerWithRedis(t)
	defer cleanupTestDatabase(t)
	defer cleanupTestRedis(t)

	router.GET("/products", server.listProductsHandler)

	// Create test product
	product := models.Product{
		Name:        "Test Product",
		Description: "Description",
		Price:       10.0,
		Stock:       5,
		Category:    "Category",
	}
	err := server.db.Create(&product).Error
	require.NoError(t, err)

	// First request - populate cache
	req1, _ := http.NewRequest("GET", "/products", nil)
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	require.Equal(t, http.StatusOK, resp1.Code)

	// Verify cache key exists
	ctx := context.Background()
	cacheKey := "products:page:1:size:10:search:"
	exists, err := server.redis.Exists(ctx, cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists)

	// Check TTL is set (should be around 5 minutes = 300 seconds)
	ttl, err := server.redis.TTL(ctx, cacheKey).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl.Seconds(), float64(0))
	assert.LessOrEqual(t, ttl.Seconds(), float64(300))
}
