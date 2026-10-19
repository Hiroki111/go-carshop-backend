package domain

import "gorm.io/gorm"

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusFailed    OrderStatus = "failed"
)

type Order struct {
	gorm.Model
	CarID      uint        `gorm:"not null;uniqueIndex:ux_orders_one_live_order_per_car,where:deleted_at IS NULL AND (status = 'pending' OR status = 'confirmed')"`
	CarName    string      `gorm:"not null"`
	UserName   string      `gorm:"not null"`
	UserID     uint        `gorm:"not null"`
	PriceCents uint        `gorm:"not null"`
	Status     OrderStatus `gorm:"type:varchar(16);not null;default:pending"`
}
