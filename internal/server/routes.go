package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/GRACENOBLE/tundra/internal/auth"
	cldinary "github.com/GRACENOBLE/tundra/internal/cloudinary"
	"github.com/GRACENOBLE/tundra/internal/database/models"
	"github.com/GRACENOBLE/tundra/internal/ratelimit"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Apply global rate limiter to all routes
	r.Use(ratelimit.GlobalLimiter())

	// Swagger documentation endpoint
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Authentication routes with strict rate limiting to prevent brute force attacks
	authRoutes := r.Group("/auth")
	authRoutes.Use(ratelimit.AuthLimiter()) // 5 requests per minute per IP
	{
		authRoutes.POST("/register", s.signUpHandler)
		authRoutes.POST("/login", s.loginHandler)
	}

	// Public product routes with API rate limiting
	productPublic := r.Group("/products")
	productPublic.Use(ratelimit.APILimiter()) // 100 requests per minute per IP
	{
		productPublic.GET("", s.listProductsHandler)
		productPublic.GET("/:id", s.getProductHandler)
	}

	// Protected product routes (require authentication and admin role)
	productsAdmin := r.Group("/products")
	productsAdmin.Use(ratelimit.APILimiter()) // Rate limiting
	productsAdmin.Use(auth.AuthMiddleware())  // Require authentication
	productsAdmin.Use(auth.AdminMiddleware()) // Require admin role
	{
		productsAdmin.POST("", s.createProductHandler)
		productsAdmin.PUT("/:id", s.updateProductHandler)
		productsAdmin.DELETE("/:id", s.deleteProductHandler)
		productsAdmin.POST("/:id/image", s.uploadProductImageHandler)
	}

	// Protected order routes (require authentication, regular users can access)
	orders := r.Group("/orders")
	orders.Use(ratelimit.APILimiter()) // Rate limiting
	orders.Use(auth.AuthMiddleware())  // Require authentication
	{
		orders.POST("", s.createOrderHandler)
		orders.GET("", s.getOrdersHandler)
	}

	return r
}

// invalidateProductCache clears all product listing cache entries
func (s *Server) invalidateProductCache() {
	if s.redis == nil {
		return
	}

	ctx := context.Background()
	// Delete all keys matching the pattern "products:*"
	iter := s.redis.Scan(ctx, 0, "products:*", 0).Iterator()
	for iter.Next(ctx) {
		s.redis.Del(ctx, iter.Val())
	}
}

// @Summary Register a new user
// @Description Create a new user account with username, email, and password. Password must be at least 8 characters with uppercase, lowercase, number, and special character.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body object{username=string,email=string,password=string} true "Signup Request" example({"username":"john123","email":"john@example.com","password":"Password123!"})
// @Success 201 {object} object{message=string,user=object{id=string,username=string,email=string,role=string}} "User registered successfully"
// @Failure 400 {object} object{error=string} "Validation error"
// @Failure 409 {object} object{error=string} "Username or email already exists"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /auth/register [post]
func (s *Server) signUpHandler(c *gin.Context) {
	// Sign up request struct
	var signUpRequest struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Parse the request body
	if err := c.ShouldBindJSON(&signUpRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
		return
	}

	// Validate username
	if err := auth.ValidateUsername(signUpRequest.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate email format
	if err := auth.ValidateEmail(signUpRequest.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate password strength
	if err := auth.ValidatePassword(signUpRequest.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := s.db.Where("username = ?", signUpRequest.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is already taken"})
		return
	}

	// Check if email already exists
	if err := s.db.Where("email = ?", signUpRequest.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is already registered"})
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signUpRequest.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	// Create the user object
	user := models.User{
		Username: signUpRequest.Username,
		Email:    signUpRequest.Email,
		Password: string(hashedPassword),
	}

	// Save the user to the database
	if err := s.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user account"})
		return
	}

	// Return success response (without password)
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// @Summary Login to user account
// @Description Authenticate user with email and password, returns JWT token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body object{email=string,password=string} true "Login Request" example({"email":"john@example.com","password":"Password123!"})
// @Success 200 {object} object{message=string,token=string,user=object{id=string,username=string,email=string,role=string}} "Login successful"
// @Failure 400 {object} object{error=string} "Validation error"
// @Failure 401 {object} object{error=string} "Invalid credentials"
// @Router /auth/login [post]
func (s *Server) loginHandler(c *gin.Context) {
	// Login request struct
	var loginRequest struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// Parse the request body
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	// Validate email format
	if err := auth.ValidateEmail(loginRequest.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Find user by email
	var user models.User
	if err := s.db.Where("email = ?", loginRequest.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Compare password with hashed password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT for the authenticated user
	token, err := auth.GenerateJWT(user.ID, user.Username, user.Email, user.Role)
	if err != nil {
		fmt.Printf("JWT Generation Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	// Successful login response with JWT
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// @Summary Create a new product
// @Description Create a new product in the catalog (Admin only). Supports optional image upload via multipart/form-data.
// @Tags Products
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param name formData string true "Product name"
// @Param description formData string true "Product description"
// @Param price formData number true "Product price (must be positive)"
// @Param stock formData integer true "Product stock (must be non-negative)"
// @Param category formData string true "Product category"
// @Param image formData file false "Product image (jpg, jpeg, png, gif, webp)"
// @Success 201 {object} object{message=string,product=models.Product} "Product created successfully"
// @Failure 400 {object} object{error=string} "Validation error or invalid image format"
// @Failure 401 {object} object{error=string} "Unauthorized"
// @Failure 403 {object} object{error=string} "Forbidden - Admin only"
// @Failure 500 {object} object{error=string} "Internal server error or image upload failed"
// @Router /products [post]
func (s *Server) createProductHandler(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	// Get form values
	name := c.PostForm("name")
	description := c.PostForm("description")
	priceStr := c.PostForm("price")
	stockStr := c.PostForm("stock")
	category := c.PostForm("category")

	// Validate required fields
	if name == "" || description == "" || priceStr == "" || stockStr == "" || category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields (name, description, price, stock, category) are required"})
		return
	}

	// Parse and validate price
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Price must be a positive number"})
		return
	}

	// Parse and validate stock
	stock, err := strconv.ParseInt(stockStr, 10, 64)
	if err != nil || stock < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stock must be a non-negative integer"})
		return
	}

	// Get user ID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User authentication required"})
		return
	}

	// Parse user ID to UUID
	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Create product
	product := models.Product{
		Name:        name,
		Description: description,
		Price:       price,
		Stock:       stock,
		Category:    category,
		UserID:      userUUID,
	}

	// Handle image upload if provided
	file, header, err := c.Request.FormFile("image")
	if err == nil && file != nil {
		defer file.Close()

		// Check if Cloudinary is available
		if s.cloudinary == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload service is not available"})
			return
		}

		// Upload to Cloudinary
		imageURL, err := s.cloudinary.UploadImage(file, header.Filename, "products")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to upload image: %v", err)})
			return
		}

		product.ImageURL = imageURL
	}

	// Save product to database
	if err := s.db.Create(&product).Error; err != nil {
		fmt.Printf("Database error creating product: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create product: %v", err)})
		return
	}

	// Invalidate product listing cache
	s.invalidateProductCache()

	c.JSON(http.StatusCreated, gin.H{
		"message": "Product created successfully",
		"product": product,
	})
}

// @Summary Update a product
// @Description Update product details (Admin only). All fields are optional.
// @Tags Products
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Product ID"
// @Param request body object{name=string,description=string,price=number,stock=integer,category=string} false "Product Update Request" example({"name":"Updated Laptop","price":899.99})
// @Success 200 {object} object{message=string,product=models.Product} "Product updated successfully"
// @Failure 400 {object} object{error=string} "Validation error"
// @Failure 401 {object} object{error=string} "Unauthorized"
// @Failure 403 {object} object{error=string} "Forbidden - Admin only"
// @Failure 404 {object} object{error=string} "Product not found"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /products/{id} [put]
func (s *Server) updateProductHandler(c *gin.Context) {
	// Get product ID from URL parameter
	id := c.Param("id")

	// Find the product by ID
	var product models.Product
	if err := s.db.Where("id = ?", id).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Update request struct - all fields are optional
	var updateRequest struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Price       *float64 `json:"price"`
		Stock       *int64   `json:"stock"`
		Category    *string  `json:"category"`
	}

	// Parse the request body
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate and update only the fields that were provided
	if updateRequest.Name != nil {
		// Name must be a non-empty string
		if len(*updateRequest.Name) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name must be a non-empty string"})
			return
		}
		product.Name = *updateRequest.Name
	}

	if updateRequest.Description != nil {
		// Description must be a non-empty string
		if len(*updateRequest.Description) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Description must be a non-empty string"})
			return
		}
		product.Description = *updateRequest.Description
	}

	if updateRequest.Price != nil {
		// Price must be a positive number
		if *updateRequest.Price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Price must be a positive number"})
			return
		}
		product.Price = *updateRequest.Price
	}

	if updateRequest.Stock != nil {
		// Stock must be a non-negative integer
		if *updateRequest.Stock < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Stock must be a non-negative integer"})
			return
		}
		product.Stock = *updateRequest.Stock
	}

	if updateRequest.Category != nil {
		// Category must be a non-empty string
		if len(*updateRequest.Category) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Category must be a non-empty string"})
			return
		}
		product.Category = *updateRequest.Category
	}

	// Save the updated product
	if err := s.db.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	// Invalidate product listing cache
	s.invalidateProductCache()

	c.JSON(http.StatusOK, gin.H{
		"message": "Product updated successfully",
		"product": product,
	})
}

// @Summary Get list of products
// @Description Get paginated list of products with optional search. Results are cached in Redis for 5 minutes.
// @Tags Products
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Items per page" default(10)
// @Param limit query int false "Items per page (alternative to pageSize)" default(10)
// @Param search query string false "Search by product name (case-insensitive partial match)"
// @Success 200 {object} object{currentPage=int,pageSize=int,totalPages=int,totalProducts=int,products=[]models.Product} "List of products"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /products [get]
func (s *Server) listProductsHandler(c *gin.Context) {
	// Get pagination parameters from query string
	page := 1
	pageSize := 10

	// Parse page parameter
	if pageParam := c.Query("page"); pageParam != "" {
		if parsedPage, err := parsePositiveInt(pageParam); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	// Parse pageSize/limit parameter (support both names)
	if pageSizeParam := c.Query("pageSize"); pageSizeParam != "" {
		if parsedSize, err := parsePositiveInt(pageSizeParam); err == nil && parsedSize > 0 {
			pageSize = parsedSize
		}
	} else if limitParam := c.Query("limit"); limitParam != "" {
		if parsedLimit, err := parsePositiveInt(limitParam); err == nil && parsedLimit > 0 {
			pageSize = parsedLimit
		}
	}

	// Get search parameter
	searchQuery := c.Query("search")

	// Generate cache key based on query parameters
	cacheKey := fmt.Sprintf("products:page:%d:size:%d:search:%s", page, pageSize, searchQuery)

	// Try to get from cache if Redis is available
	if s.redis != nil {
		ctx := context.Background()
		cachedData, err := s.redis.Get(ctx, cacheKey).Result()
		if err == nil && cachedData != "" {
			// Cache hit - return cached data
			var cachedResponse map[string]interface{}
			if err := json.Unmarshal([]byte(cachedData), &cachedResponse); err == nil {
				c.JSON(http.StatusOK, cachedResponse)
				return
			}
		}
	}

	// Cache miss or Redis unavailable - fetch from database
	// Calculate offset for pagination
	offset := (page - 1) * pageSize

	// Build query with optional search filter
	query := s.db.Model(&models.Product{})
	if searchQuery != "" {
		// Case-insensitive partial match search on product name
		query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+searchQuery+"%")
	}

	// Get total count of products (with search filter if applicable)
	var totalProducts int64
	if err := query.Count(&totalProducts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
		return
	}

	// Calculate total pages
	totalPages := int(totalProducts) / pageSize
	if int(totalProducts)%pageSize != 0 {
		totalPages++
	}

	// If total is 0, totalPages should be 0
	if totalProducts == 0 {
		totalPages = 0
	}

	// Get products for current page (with search filter if applicable)
	var products []models.Product
	if err := query.Offset(offset).Limit(pageSize).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	// Prepare response
	response := gin.H{
		"currentPage":   page,
		"pageSize":      len(products),
		"totalPages":    totalPages,
		"totalProducts": totalProducts,
		"products":      products,
	}

	// Cache the response if Redis is available
	if s.redis != nil {
		ctx := context.Background()
		responseJSON, err := json.Marshal(response)
		if err == nil {
			// Cache for 5 minutes
			s.redis.Set(ctx, cacheKey, responseJSON, 5*time.Minute)
		}
	}

	// Return response
	c.JSON(http.StatusOK, response)
}

// @Summary Get product by ID
// @Description Get detailed information about a specific product
// @Tags Products
// @Produce json
// @Param id path string true "Product ID (UUID)"
// @Success 200 {object} models.Product "Product details"
// @Failure 404 {object} object{error=string} "Product not found"
// @Router /products/{id} [get]
func (s *Server) getProductHandler(c *gin.Context) {
	// Get product ID from URL parameter
	productID := c.Param("id")

	// Find product by ID
	var product models.Product
	if err := s.db.Where("id = ?", productID).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Return product details
	c.JSON(http.StatusOK, product)
}

// @Summary Delete a product
// @Description Delete a product from the catalog (Admin only). Invalidates product listing cache.
// @Tags Products
// @Security Bearer
// @Param id path string true "Product ID (UUID)"
// @Success 200 {object} object{message=string} "Product deleted successfully"
// @Failure 401 {object} object{error=string} "Unauthorized"
// @Failure 403 {object} object{error=string} "Forbidden - Admin only"
// @Failure 404 {object} object{error=string} "Product not found"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /products/{id} [delete]
func (s *Server) deleteProductHandler(c *gin.Context) {
	// Get product ID from URL parameter
	productID := c.Param("id")

	// Find product by ID first to check if it exists
	var product models.Product
	if err := s.db.Where("id = ?", productID).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Delete image from Cloudinary if it exists
	if product.ImageURL != "" && s.cloudinary != nil {
		publicID := cldinary.ExtractPublicID(product.ImageURL)
		if publicID != "" {
			// Delete from Cloudinary (don't fail if this fails)
			_ = s.cloudinary.DeleteImage(publicID)
		}
	}

	// Delete the product
	if err := s.db.Delete(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	// Invalidate product listing cache
	s.invalidateProductCache()

	// Return success message
	c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
}

// @Summary Upload product image
// @Description Upload or update a product's image (Admin only). Accepts image files in jpg, jpeg, png, gif, or webp format.
// @Tags Products
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param id path string true "Product ID"
// @Param image formData file true "Product image file (jpg, jpeg, png, gif, webp, max 10MB)"
// @Success 200 {object} object{message=string,imageUrl=string} "Image uploaded successfully"
// @Failure 400 {object} object{error=string} "Invalid file format or upload error"
// @Failure 401 {object} object{error=string} "Unauthorized"
// @Failure 403 {object} object{error=string} "Forbidden - Admin only"
// @Failure 404 {object} object{error=string} "Product not found"
// @Failure 500 {object} object{error=string} "Image upload service unavailable"
// @Router /products/{id}/image [post]
func (s *Server) uploadProductImageHandler(c *gin.Context) {
	// Get product ID from URL
	productID := c.Param("id")

	// Find product
	var product models.Product
	if err := s.db.Where("id = ?", productID).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Check if Cloudinary is available
	if s.cloudinary == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload service is not available"})
		return
	}

	// Get the uploaded file
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided"})
		return
	}
	defer file.Close()

	// Delete old image if it exists
	if product.ImageURL != "" {
		publicID := cldinary.ExtractPublicID(product.ImageURL)
		if publicID != "" {
			// Don't fail if deletion fails
			_ = s.cloudinary.DeleteImage(publicID)
		}
	}

	// Upload new image
	imageURL, err := s.cloudinary.UploadImage(file, header.Filename, "products")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to upload image: %v", err)})
		return
	}

	// Update product with new image URL
	product.ImageURL = imageURL
	if err := s.db.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product with image URL"})
		return
	}

	// Invalidate product cache
	s.invalidateProductCache()

	c.JSON(http.StatusOK, gin.H{
		"message":  "Image uploaded successfully",
		"imageUrl": imageURL,
	})
}

// @Summary Create a new order
// @Description Create a new order for the authenticated user with one or more products. This endpoint validates product availability, checks stock levels, and updates inventory atomically. All operations are performed within a database transaction to ensure data consistency.
// @Tags Orders
// @Accept json
// @Produce json
// @Security Bearer
// @Param order body object{items=[]object{productId=string,quantity=int}} true "Order items with product IDs and quantities"
// @Success 201 {object} models.Order "Order created successfully with full details including order products"
// @Failure 400 {object} object{error=string} "Invalid request body, empty order, or insufficient stock"
// @Failure 401 {object} object{error=string} "User not authenticated"
// @Failure 404 {object} object{error=string} "One or more products not found"
// @Failure 500 {object} object{error=string} "Failed to create order or update stock"
// @Router /orders [post]
func (s *Server) createOrderHandler(c *gin.Context) {
	// Get user ID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Parse request body
	var orderItems []struct {
		ProductID string `json:"productId" binding:"required"`
		Quantity  int    `json:"quantity" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&orderItems); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(orderItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order must contain at least one item"})
		return
	}

	// Start database transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	// Validate products and check stock
	var totalPrice float64
	var orderProducts []models.OrderProduct

	for _, item := range orderItems {
		var product models.Product

		// Find product and lock row for update to prevent race conditions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", item.ProductID).First(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Product with ID %s not found", item.ProductID)})
			return
		}

		// Check stock availability
		if product.Stock < int64(item.Quantity) {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Insufficient stock for product: %s (available: %d, requested: %d)", product.Name, product.Stock, item.Quantity)})
			return
		}

		// Calculate item total and add to order total
		itemTotal := product.Price * float64(item.Quantity)
		totalPrice += itemTotal

		// Update product stock
		product.Stock -= int64(item.Quantity)
		if err := tx.Save(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
			return
		}

		// Store order product info for later creation
		orderProducts = append(orderProducts, models.OrderProduct{
			ProductID: product.ID,
			Quantity:  item.Quantity,
			Price:     product.Price, // Store price at time of order
		})
	}

	// Create order
	order := models.Order{
		UserID:      userID.(uuid.UUID),
		Description: fmt.Sprintf("Order with %d item(s)", len(orderItems)),
		TotalPrice:  totalPrice,
		Status:      "pending",
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	// Create order products (join table entries)
	for i := range orderProducts {
		orderProducts[i].OrderID = order.ID
	}

	if err := tx.Create(&orderProducts).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order items"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Load order products with product details for response
	var createdOrder models.Order
	if err := s.db.Preload("OrderProducts.Product").First(&createdOrder, order.ID).Error; err != nil {
		// Order was created successfully, but we couldn't load it
		// Return basic order info
		c.JSON(http.StatusCreated, order)
		return
	}

	// Return created order with full details
	c.JSON(http.StatusCreated, createdOrder)
}

// @Summary Get user's orders
// @Description Retrieve all orders for the authenticated user, ordered by creation date (newest first). Returns an empty array if the user has no orders.
// @Tags Orders
// @Produce json
// @Security Bearer
// @Success 200 {array} models.Order "List of user's orders (may be empty)"
// @Failure 401 {object} object{error=string} "User not authenticated"
// @Failure 500 {object} object{error=string} "Failed to retrieve orders from database"
// @Router /orders [get]
func (s *Server) getOrdersHandler(c *gin.Context) {
	// Get user ID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Query orders for the authenticated user
	var orders []models.Order
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve orders"})
		return
	}

	// Return orders (empty array if no orders found)
	c.JSON(http.StatusOK, orders)
}

// Helper function to parse positive integers from string
func parsePositiveInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
