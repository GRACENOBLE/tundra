package models

import "github.com/google/uuid"

type User struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Username string    `gorm:"uniqueIndex;not null" json:"username"`
	Email    string    `gorm:"uniqueIndex;not null" json:"email"`
	Password string    `gorm:"not null" json:"-"`
}
