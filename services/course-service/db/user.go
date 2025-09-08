package db

type User struct {
	ID      uint `gorm:"primaryKey"`
	Name    string
	TgID    string `gorm:"column:tgid"`
	Email   string // `gorm:"uniqueIndex"` // было: Birthdate string
	IsAdmin bool   `gorm:"default:false"`
}
