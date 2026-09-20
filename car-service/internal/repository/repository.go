package repository

import (
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Migrate() error {
	return r.db.AutoMigrate(&domain.Car{})
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) withTx(tx *gorm.DB) *Repository {
	if tx == nil {
		return r
	}
	return &Repository{db: tx}
}
