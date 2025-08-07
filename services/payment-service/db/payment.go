package db

import "time"

type Payment struct {
	ID        string
	TgID      string `gorm:"column:tgid"`
	CourseID  string
	Status    string
	CreatedAt time.Time
}
