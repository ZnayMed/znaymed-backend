package db

type User struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	TgID      string `gorm:"column:tgid"`
	Birthdate string
}
