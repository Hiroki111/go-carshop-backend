package domain

import (
	"time"
)

type EventType string

const (
	EventTypeOrderConfirmed EventType = "order.confirmed"
	EventTypeOrderCancelled EventType = "order.cancelled"
)

type OutboxEvent struct {
	ID          uint      `gorm:"primaryKey;index:idx_outbox_events_unpublished,where:published_at IS NULL"`
	CreatedAt   time.Time `gorm:"not null"`
	EventID     string    `gorm:"type:varchar(36);not null;uniqueIndex"`
	EventType   EventType `gorm:"type:varchar(32);not null"`
	OrderID     uint      `gorm:"not null"`
	Payload     string    `gorm:"type:text;not null"`
	PublishedAt *time.Time
}

type OrderEventPayload struct {
	EventID    string    `json:"event_id"`
	EventType  EventType `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	OrderID    uint      `json:"order_id"`
	UserID     uint      `json:"user_id"`
	CarID      uint      `json:"car_id"`
	CarName    string    `json:"car_name"`
	PriceCents uint      `json:"price_cents"`
}
