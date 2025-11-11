package models

import "github.com/google/uuid"

// OrderProduct represents the join table between orders and products
type OrderProduct struct {
	OrderID   uuid.UUID `gorm:"type:uuid;primaryKey" json:"order_id"`
	ProductID uuid.UUID `gorm:"type:uuid;primaryKey" json:"product_id"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	Price     float64   `gorm:"not null" json:"price"` // Price at time of order
	Product   Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
}
