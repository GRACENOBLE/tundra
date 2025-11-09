package models

import "github.com/google/uuid"

type Order struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	Description string    `gorm:"not null" json:"description"`
	TotalPrice  float64   `gorm:"not null" json:"total_price"`
	Status      string    `gorm:"not null" json:"status"`
	Products    []Product `gorm:"many2many:order_products;" json:"products"`
}
