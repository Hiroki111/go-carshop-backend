package processor

import "time"

const (
	EventConfirmed = "order.confirmed"
	EventCancelled = "order.cancelled"
)

var validEventTypes = map[string]bool{
	EventConfirmed: true,
	EventCancelled: true,
}

type OrderEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	OrderID    uint      `json:"order_id"`
	UserID     uint      `json:"user_id"`
	CarID      uint      `json:"car_id"`
	CarName    string    `json:"car_name"`
	PriceCents uint      `json:"price_cents"`
}
