package domain

import "gorm.io/gorm"

type Order struct {
	gorm.Model
	CarID      uint `gorm:"not null"`
	UserID     uint `gorm:"not null"`
	PriceCents uint `gorm:"not null"`
}
