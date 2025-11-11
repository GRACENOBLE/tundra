package models

import (
	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `gorm:"not null" json:"description"`
	Price       float64   `gorm:"not null" json:"price"`
	Stock       int64     `gorm:"not null" json:"stock"`
	Category    string    `gorm:"not null" json:"category"`
	ImageURL    string    `gorm:"default:null" json:"imageUrl,omitempty"`
	UserID      uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
}
