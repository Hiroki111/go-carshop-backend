package domain

import "time"

type ProcessedEvent struct {
	EventID     string    `gorm:"primaryKey"`
	ProcessedAt time.Time `gorm:"not null"`
}
