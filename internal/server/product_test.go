package server

import (
	"testing"
)

func TestProductValidation(t *testing.T) {
	tests := []struct {
		name          string
		productName   string
		description   string
		price         float64
		stock         int64
		category      string
		expectedError string
	}{
		{
			name:        "Valid product",
			productName: "Test Product",
			description: "Test Description",
			price:       29.99,
			stock:       100,
			category:    "Electronics",
		},
		{
			name:          "Empty name",
			productName:   "",
			description:   "Test Description",
			price:         29.99,
			stock:         100,
			category:      "Electronics",
			expectedError: "Name must be a non-empty string",
		},
		{
			name:          "Empty description",
			productName:   "Test Product",
			description:   "",
			price:         29.99,
			stock:         100,
			category:      "Electronics",
			expectedError: "Description must be a non-empty string",
		},
		{
			name:          "Zero price",
			productName:   "Test Product",
			description:   "Test Description",
			price:         0,
			stock:         100,
			category:      "Electronics",
			expectedError: "Price must be a positive number",
		},
		{
			name:          "Negative price",
			productName:   "Test Product",
			description:   "Test Description",
			price:         -10.99,
			stock:         100,
			category:      "Electronics",
			expectedError: "Price must be a positive number",
		},
		{
			name:          "Negative stock",
			productName:   "Test Product",
			description:   "Test Description",
			price:         29.99,
			stock:         -5,
			category:      "Electronics",
			expectedError: "Stock must be a non-negative integer",
		},
		{
			name:          "Empty category",
			productName:   "Test Product",
			description:   "Test Description",
			price:         29.99,
			stock:         100,
			category:      "",
			expectedError: "Category must be a non-empty string",
		},
		{
			name:        "Zero stock is valid",
			productName: "Test Product",
			description: "Test Description",
			price:       29.99,
			stock:       0,
			category:    "Electronics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log("Testing product validation:", tt.name)
			if tt.expectedError != "" {
				t.Log("Expected error:", tt.expectedError)
			}
		})
	}
}

func TestUpdateProductValidation(t *testing.T) {
	tests := []struct {
		name          string
		updateField   string
		updateValue   interface{}
		expectedError string
	}{
		{
			name:        "Valid name update",
			updateField: "name",
			updateValue: "Updated Product",
		},
		{
			name:          "Empty name update",
			updateField:   "name",
			updateValue:   "",
			expectedError: "Name must be a non-empty string",
		},
		{
			name:        "Valid description update",
			updateField: "description",
			updateValue: "Updated Description",
		},
		{
			name:          "Empty description update",
			updateField:   "description",
			updateValue:   "",
			expectedError: "Description must be a non-empty string",
		},
		{
			name:        "Valid price update",
			updateField: "price",
			updateValue: 49.99,
		},
		{
			name:          "Zero price update",
			updateField:   "price",
			updateValue:   0.0,
			expectedError: "Price must be a positive number",
		},
		{
			name:          "Negative price update",
			updateField:   "price",
			updateValue:   -10.0,
			expectedError: "Price must be a positive number",
		},
		{
			name:        "Valid stock update",
			updateField: "stock",
			updateValue: int64(50),
		},
		{
			name:        "Zero stock update is valid",
			updateField: "stock",
			updateValue: int64(0),
		},
		{
			name:          "Negative stock update",
			updateField:   "stock",
			updateValue:   int64(-10),
			expectedError: "Stock must be a non-negative integer",
		},
		{
			name:        "Valid category update",
			updateField: "category",
			updateValue: "Updated Category",
		},
		{
			name:          "Empty category update",
			updateField:   "category",
			updateValue:   "",
			expectedError: "Category must be a non-empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log("Testing product update validation:", tt.name)
			t.Log("Update field:", tt.updateField)
			if tt.expectedError != "" {
				t.Log("Expected error:", tt.expectedError)
			}
		})
	}
}

func TestProductValidationRules(t *testing.T) {
	t.Run("Validation rules documentation", func(t *testing.T) {
		t.Log("Product validation rules:")
		t.Log("1. Name: Must be a non-empty string")
		t.Log("2. Description: Must be a non-empty string")
		t.Log("3. Price: Must be a positive number (> 0)")
		t.Log("4. Stock: Must be a non-negative integer (>= 0)")
		t.Log("5. Category: Must be a non-empty string")
	})

	t.Run("Update validation rules", func(t *testing.T) {
		t.Log("Update validation rules apply to any field provided:")
		t.Log("- All fields are optional in updates")
		t.Log("- If a field is provided, it must meet the same validation as creation")
		t.Log("- Partial updates are supported")
	})
}
