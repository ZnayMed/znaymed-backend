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
		&User{}, &Subject{}, &Section{}, &Topic{}, &UserSection{},
	); err != nil {
		return nil, err
	}

	var cnt int64
	db.Model(&Subject{}).Count(&cnt)
	if cnt == 0 {
		if err := seedInitialData(db); err != nil {
			return nil, err
		}
		log.Println("📥 База заполнена начальными предметами/темами")
	}

	return &Database{DB: db}, nil
}
