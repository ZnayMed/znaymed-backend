package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

	var gdb *gorm.DB
	var err error

	for i := 0; i < 10; i++ {
		gdb, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("DB not ready (%v); retry in 3s...", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := gdb.AutoMigrate(
		&User{}, &Subject{}, &Section{}, &Topic{}, &UserSection{},
	); err != nil {
		return nil, err
	}

	var cnt int64
	gdb.Model(&Subject{}).Count(&cnt)
	if cnt == 0 {
		paths := []string{
			os.Getenv("ANATOMY_JSON"),
			os.Getenv("HISTOLOGY_JSON"),
		}

		var importPaths []string
		for _, p := range paths {
			if p != "" {
				importPaths = append(importPaths, p)
			}
		}

		if len(importPaths) == 0 {
			return nil, fmt.Errorf("empty DB and neither ANATOMY_JSON nor HISTOLOGY_JSON is set — aborting to avoid loading test seed")
		}

		for _, path := range importPaths {
			log.Printf("Empty DB: importing subjects from JSON: %s ...", path)
			if err := ImportSubjectsFromFile(gdb, path); err != nil {
				return nil, fmt.Errorf("import failed from %s: %w", path, err)
			}
		}
		log.Println("Import finished successfully")
	}

	return &Database{DB: gdb}, nil
}
