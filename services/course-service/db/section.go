package db

type Section struct {
	ID          uint   `gorm:"primaryKey"`
	SubjectID   uint   `gorm:"not null"`
	Title       string `gorm:"not null"`
	Description string
	Topics      []Topic `gorm:"foreignKey:SectionID"`
}
