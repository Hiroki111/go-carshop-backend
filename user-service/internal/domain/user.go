package domain

import "gorm.io/gorm"

type UserRole string

const (
	AdminRole    UserRole = "admin"
	CustomerRole UserRole = "customer"
)

type User struct {
	gorm.Model
	UserName string   `gorm:"uniqueIndex;not null"`
	Password string   `gorm:"not null"`
	Role     UserRole `gorm:"not null;check:role IN ('admin','customer')"`
}
