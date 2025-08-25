package db

import "time"

type Payment struct {
	ID              string `gorm:"primaryKey"`
	TgID            string `gorm:"column:tgid;index:idx_tgid_course_pending,priority:1"`
	CourseID        string `gorm:"index:idx_tgid_course_pending,priority:2"`
	Status          string `gorm:"index"` // PENDING | PAID | CANCELED
	Amount          int64  // в копейках
	Currency        string // "RUB"
	Description     string
	Provider        string // "yookassa"
	ProviderID      string `gorm:"index"` // id платежа у ЮKassa
	ConfirmationURL string
	ExpiresAt       *time.Time
	IdempotenceKey  string `gorm:"index"`
	CreatedAt       time.Time
}
