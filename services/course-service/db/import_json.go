package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gorm.io/gorm"
)

type seedSubject struct {
	Title    string        `json:"Title"`
	Sections []seedSection `json:"Sections"`
}
type seedSection struct {
	Title       string      `json:"Title"`
	Description string      `json:"Description"`
	PriceKopeck int64       `json:"PriceKopeck"`
	Topics      []seedTopic `json:"Topics"`
}
type seedTopic struct {
	Title       string `json:"Title"`
	Description string `json:"Description"`
	TgID        string `json:"TgID"`
	MindmapURL  string `json:"MindmapURL"`
}

func ImportSubjectsFromFile(db *gorm.DB, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open json: %w", err)
	}
	defer f.Close()
	return ImportSubjectsFromReader(db, f)
}

func ImportSubjectsFromReader(db *gorm.DB, r io.Reader) error {
	var subj seedSubject
	if err := json.NewDecoder(r).Decode(&subj); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return importOneSubject(db, subj)
}

func importOneSubject(db *gorm.DB, s seedSubject) error {
	title := strings.TrimSpace(s.Title)
	if title == "" {
		return errors.New("subject.Title is empty")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var subj Subject
		if err := tx.Where("title = ?", title).First(&subj).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				subj = Subject{Title: title}
				if err := tx.Create(&subj).Error; err != nil {
					return fmt.Errorf("create subject %q: %w", title, err)
				}
			} else {
				return fmt.Errorf("find subject %q: %w", title, err)
			}
		}

		for _, sec := range s.Sections {
			secTitle := strings.TrimSpace(sec.Title)
			if secTitle == "" {
				return fmt.Errorf("section without Title in subject %q", title)
			}

			var section Section
			if err := tx.
				Where("subject_id = ? AND title = ?", subj.ID, secTitle).
				First(&section).Error; err != nil {

				if errors.Is(err, gorm.ErrRecordNotFound) {
					section = Section{
						SubjectID:   subj.ID,
						Title:       secTitle,
						Description: strings.TrimSpace(sec.Description),
						PriceKopeck: sec.PriceKopeck,
					}
					if err := tx.Create(&section).Error; err != nil {
						return fmt.Errorf("create section %q: %w", secTitle, err)
					}
				} else {
					return fmt.Errorf("find section %q: %w", secTitle, err)
				}
			} else {
				section.Description = strings.TrimSpace(sec.Description)
				section.PriceKopeck = sec.PriceKopeck
				if err := tx.Save(&section).Error; err != nil {
					return fmt.Errorf("update section %q: %w", secTitle, err)
				}
			}

			for _, t := range sec.Topics {
				tTitle := strings.TrimSpace(t.Title)
				if tTitle == "" {
					return fmt.Errorf("topic without Title in section %q", secTitle)
				}

				var topic Topic
				if err := tx.
					Where("section_id = ? AND title = ?", section.ID, tTitle).
					First(&topic).Error; err != nil {

					if errors.Is(err, gorm.ErrRecordNotFound) {
						topic = Topic{
							SectionID:   section.ID,
							Title:       tTitle,
							Description: strings.TrimSpace(t.Description),
							TgID:        strings.TrimSpace(t.TgID),
							MindmapURL:  strings.TrimSpace(t.MindmapURL),
						}
						if err := tx.Create(&topic).Error; err != nil {
							return fmt.Errorf("create topic %q: %w", tTitle, err)
						}
					} else {
						return fmt.Errorf("find topic %q: %w", tTitle, err)
					}
				} else {
					topic.Description = strings.TrimSpace(t.Description)
					topic.TgID = strings.TrimSpace(t.TgID)
					topic.MindmapURL = strings.TrimSpace(t.MindmapURL)
					if err := tx.Save(&topic).Error; err != nil {
						return fmt.Errorf("update topic %q: %w", tTitle, err)
					}
				}
			}
		}
		return nil
	})
}
