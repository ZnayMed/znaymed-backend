package db

type Subject struct {
	ID          uint      `gorm:"primaryKey"`
	Title       string    `gorm:"not null"`
	Sections    []Section `gorm:"foreignKey:SubjectID"`
	Description string
}
