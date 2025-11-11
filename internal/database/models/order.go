package models

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	Description   string         `gorm:"not null" json:"description"`
	TotalPrice    float64        `gorm:"not null" json:"total_price"`
	Status        string         `gorm:"not null" json:"status"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	OrderProducts []OrderProduct `gorm:"foreignKey:OrderID" json:"order_products,omitempty"`
}
