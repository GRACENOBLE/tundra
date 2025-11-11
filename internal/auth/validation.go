package auth

import (
	"errors"
	"regexp"
	"unicode"
)

// ValidatePassword checks if password meets all requirements
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must include at least one uppercase letter (A-Z)")
	}
	if !hasLower {
		return errors.New("password must include at least one lowercase letter (a-z)")
	}
	if !hasNumber {
		return errors.New("password must include at least one number (0-9)")
	}
	if !hasSpecial {
		return errors.New("password must include at least one special character (e.g., !@#$%^&*)")
	}

	return nil
}

// ValidateUsername checks if username is alphanumeric only
func ValidateUsername(username string) error {
	if username == "" {
		return errors.New("username is required")
	}

	// Check if username contains only alphanumeric characters
	alphanumericRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !alphanumericRegex.MatchString(username) {
		return errors.New("username must be alphanumeric (letters and numbers only, no special characters or spaces)")
	}

	return nil
}

// ValidateEmail checks if email format is valid
func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}

	// Basic email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return errors.New("email must be a valid email address format (e.g., user@example.com)")
	}

	return nil
}
