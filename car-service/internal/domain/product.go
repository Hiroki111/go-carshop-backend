package domain

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;not null"`
	PriceCents  uint   `gorm:"not null"`
	Version     uint   `gorm:"not null;default:1"`
	IsAvailable bool   `gorm:"not null;"`
}
