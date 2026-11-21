package repository

import (
	"time"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/domain"
)

func (r *Repository) IsEventProcessed(eventId string) (bool, error) {
	var events []domain.ProcessedEvent

	if err := r.db.Where("event_id = ?", eventId).Limit(1).Find(&events).Error; err != nil {
		return false, err
	}

	return len(events) > 0, nil
}

func (r *Repository) CreateProcessedEvent(orderEventId string) error {
	return r.db.Create(&domain.ProcessedEvent{
		EventID:     orderEventId,
		ProcessedAt: time.Now().UTC(),
	}).Error
}
