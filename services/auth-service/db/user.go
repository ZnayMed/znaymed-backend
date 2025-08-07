package db

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
)

type User struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	TgID      string `gorm:"column:tgid"`
	Birthdate string
}

func (d *Database) SaveUser(name, tgid, birthdate string) error {
	var existing User
	if err := d.DB.Where("tgid = ?", tgid).First(&existing).Error; err == nil {
		return fmt.Errorf("пользователь уже зарегистрирован")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("ошибка при проверке пользователя: %w", err)
	}

	user := User{
		Name:      name,
		TgID:      tgid,
		Birthdate: birthdate,
	}
	if err := d.DB.Create(&user).Error; err != nil {
		fmt.Println("GORM ошибка при сохранении пользователя:", err)
		return err
	}
	return nil
}
