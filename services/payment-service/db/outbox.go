package db

import (
	"time"
)

type Outbox struct {
	ID        uint      `gorm:"primaryKey"`
	EventType string    `gorm:"not null"`
	Payload   []byte    `gorm:"type:jsonb"`
	Sent      bool      `gorm:"default:false"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
