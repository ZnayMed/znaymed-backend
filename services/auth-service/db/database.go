package db

import (
	"fmt"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"time"
)

type Database struct {
	DB *gorm.DB
}

func NewDatabase() (*Database, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file, continue with docker vars: %v", err)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	var db *gorm.DB
	var err error

	// Пытаемся подключиться до 30 секунд, каждые 3 сек.
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("DB not ready (%v); retry in 3s...", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := db.AutoMigrate(
		&User{},
	); err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}
