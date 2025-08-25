package db

import "time"

type Payment struct {
	ID              string `gorm:"primaryKey"`
	TgID            string `gorm:"column:tgid;index:idx_tgid_course_pending,priority:1"`
	CourseID        string `gorm:"index:idx_tgid_course_pending,priority:2"`
	Status          string `gorm:"index"`
	Amount          int64
	Currency        string
	Description     string
	Provider        string
	ProviderID      string `gorm:"index"`
	ConfirmationURL string
	ExpiresAt       *time.Time
	IdempotenceKey  string `gorm:"index"`
	CreatedAt       time.Time
}
