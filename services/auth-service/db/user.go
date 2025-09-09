package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type User struct {
	ID      uint `gorm:"primaryKey"`
	Name    string
	TgID    string `gorm:"column:tgid"`
	Email   string // `gorm:"uniqueIndex"` // было: Birthdate string
	IsAdmin bool   `gorm:"default:false"`
}

func (d *Database) GetUserByTGIDHash(tgidHash string) (User, error) {
	var u User
	err := d.DB.Where("tgid = ?", tgidHash).First(&u).Error
	return u, err
}

func (d *Database) UserExists(tgid string) (bool, error) {
	var cnt int64
	if err := d.DB.Model(&User{}).Where("tgid = ?", tgid).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (d *Database) SaveUser(name, tgid, email string) error {
	// Проверка на существование по TGID
	var existing User
	if err := d.DB.Where("tgid = ?", tgid).First(&existing).Error; err == nil {
		return fmt.Errorf("пользователь уже зарегистрирован")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("ошибка при проверке пользователя: %w", err)
	}

	//// (Опционально) проверка уникальности email
	//var byEmail User
	//if err := d.DB.Where("email = ?", email).First(&byEmail).Error; err == nil {
	//	return fmt.Errorf("email уже занят")
	//} else if !errors.Is(err, gorm.ErrRecordNotFound) {
	//	return fmt.Errorf("ошибка при проверке email: %w", err)
	//}

	user := User{
		Name:  name,
		TgID:  tgid,
		Email: email,
	}
	if err := d.DB.Create(&user).Error; err != nil {
		fmt.Println("GORM ошибка при сохранении пользователя:", err)
		return err
	}
	return nil
}

func (d *Database) IsAdminByTGIDHash(tgidHash string) (bool, error) {
	var u User
	if err := d.DB.
		Select("is_admin").
		Where("tgid = ?", tgidHash).
		First(&u).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("db query error: %w", err)
	}
	return u.IsAdmin, nil
}
