package service

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
)

func (s *Service) newOutboxEvent(eventType domain.EventType, order domain.Order) (domain.OutboxEvent, error) {
	payload := domain.OrderEventPayload{
		EventID:    uuid.New().String(),
		EventType:  eventType,
		Version:    1,
		OccurredAt: time.Now().UTC(),
		OrderID:    order.ID,
		UserID:     order.UserID,
		CarID:      order.CarID,
		CarName:    order.CarName,
		PriceCents: order.PriceCents,
	}
	payloadInBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		EventID:   payload.EventID,
		EventType: payload.EventType,
		OrderID:   payload.OrderID,
		Payload:   string(payloadInBytes),
	}, nil
}
