package repository

import (
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
)

func (r *Repository) CreateOutboxEvent(event domain.OutboxEvent) error {
	return r.db.Create(&event).Error
}
