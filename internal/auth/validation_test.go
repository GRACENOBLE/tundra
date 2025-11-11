package auth

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "Valid password",
			password: "Password123!",
			wantErr:  false,
		},
		{
			name:     "Too short",
			password: "Pass1!",
			wantErr:  true,
		},
		{
			name:     "No uppercase",
			password: "password123!",
			wantErr:  true,
		},
		{
			name:     "No lowercase",
			password: "PASSWORD123!",
			wantErr:  true,
		},
		{
			name:     "No number",
			password: "Password!",
			wantErr:  true,
		},
		{
			name:     "No special character",
			password: "Password123",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "Valid username",
			username: "user123",
			wantErr:  false,
		},
		{
			name:     "Valid uppercase",
			username: "User123",
			wantErr:  false,
		},
		{
			name:     "Empty username",
			username: "",
			wantErr:  true,
		},
		{
			name:     "Username with spaces",
			username: "user 123",
			wantErr:  true,
		},
		{
			name:     "Username with special characters",
			username: "user@123",
			wantErr:  true,
		},
		{
			name:     "Username with underscore",
			username: "user_123",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "Valid email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "Valid email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "Empty email",
			email:   "",
			wantErr: true,
		},
		{
			name:    "No @ symbol",
			email:   "userexample.com",
			wantErr: true,
		},
		{
			name:    "No domain",
			email:   "user@",
			wantErr: true,
		},
		{
			name:    "No TLD",
			email:   "user@example",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
