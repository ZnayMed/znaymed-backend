package db

import "fmt"

type User struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	TgID      string `gorm:"column:tgid"`
	Birthdate string
}

func (d *Database) SaveUser(name, tgid, birthdate string) error {
	user := User{
		Name:      name,
		TgID:      tgid,
		Birthdate: birthdate,
	}
	err := d.DB.Create(&user).Error
	if err != nil {
		fmt.Println("GORM ошибка при сохранении пользователя:", err)
	}
	return err
}
