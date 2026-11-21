package repository

import (
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Migrate() error {
	return r.db.AutoMigrate(&domain.ProcessedEvent{})
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}
