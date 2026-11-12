package repository

import (
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
)

func (r *Repository) CreateOutboxEvent(event domain.OutboxEvent) error {
	return r.db.Create(&event).Error
}

func (r *Repository) GetUnpublishedOutboxEvents(batchSize int) ([]domain.OutboxEvent, error) {
	var events []domain.OutboxEvent

	result := r.db.Model(&domain.OutboxEvent{}).
		Where("published_at IS NULL").
		Order("id").
		Limit(batchSize).
		Find(&events)

	return events, result.Error
}

func (r *Repository) MarkOutboxEventPublished(id uint) error {
	return r.db.Model(&domain.OutboxEvent{}).
		Where("id = ?", id).
		Update("published_at", time.Now().UTC()).Error
}
