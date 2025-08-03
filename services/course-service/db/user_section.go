package db

import "time"

type UserSection struct {
	UserID      uint      `gorm:"primaryKey"`
	SectionID   uint      `gorm:"primaryKey"`
	PurchasedAt time.Time `gorm:"autoCreateTime"`
}
