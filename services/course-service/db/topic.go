package db

type Topic struct {
	ID          uint `gorm:"primaryKey"`
	SectionID   uint
	Title       string `gorm:"not null"`
	Description string
	TgID        string `gorm:"not null"`
	MindmapURL  string `gorm:"not null"`
}
