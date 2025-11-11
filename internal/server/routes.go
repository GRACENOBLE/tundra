package server

import (
	"fmt"
	"net/http"
	"tundra/internal/auth"
	"tundra/internal/database/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	authRoutes := r.Group("/auth")
	{
		authRoutes.POST("/register", s.signUpHandler)
		authRoutes.POST("/login", s.loginHandler)
	}

	// Public product routes (no authentication required)
	r.GET("/products", s.listProductsHandler)

	// Protected product routes (require authentication and admin role)
	productsAdmin := r.Group("/products")
	productsAdmin.Use(auth.AuthMiddleware())  // Require authentication
	productsAdmin.Use(auth.AdminMiddleware()) // Require admin role
	{
		productsAdmin.POST("/", s.createProductHandler)
		productsAdmin.PUT("/:id", s.updateProductHandler)
	}

	return r
}

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

func (s *Server) createProductHandler(c *gin.Context) {
	var productRequest struct {
		Name        string  `json:"name" binding:"required"`
		Description string  `json:"description" binding:"required"`
		Price       float64 `json:"price" binding:"required"`
		Stock       int64   `json:"stock" binding:"required"`
		Category    string  `json:"category" binding:"required"`
	}

	if err := c.ShouldBindJSON(&productRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
		return
	}

	// Validate name must be non-empty
	if len(productRequest.Name) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name must be a non-empty string"})
		return
	}

	// Validate description must be non-empty
	if len(productRequest.Description) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Description must be a non-empty string"})
		return
	}

	// Validate price must be positive
	if productRequest.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Price must be a positive number"})
		return
	}

	// Validate stock must be non-negative
	if productRequest.Stock < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stock must be a non-negative integer"})
		return
	}

	// Validate category must be non-empty
	if len(productRequest.Category) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category must be a non-empty string"})
		return
	}

	product := models.Product{
		Name:        productRequest.Name,
		Description: productRequest.Description,
		Price:       productRequest.Price,
		Stock:       productRequest.Stock,
		Category:    productRequest.Category,
	}

	if err := s.db.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Product created successfully",
		"product": product,
	})
}

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

	c.JSON(http.StatusOK, gin.H{
		"message": "Product updated successfully",
		"product": product,
	})
}

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

	// Calculate offset for pagination
	offset := (page - 1) * pageSize

	// Get total count of products
	var totalProducts int64
	if err := s.db.Model(&models.Product{}).Count(&totalProducts).Error; err != nil {
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

	// Get products for current page
	var products []models.Product
	if err := s.db.Offset(offset).Limit(pageSize).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	// Return paginated response
	c.JSON(http.StatusOK, gin.H{
		"currentPage":   page,
		"pageSize":      len(products),
		"totalPages":    totalPages,
		"totalProducts": totalProducts,
		"products":      products,
	})
}

// Helper function to parse positive integers from string
func parsePositiveInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
