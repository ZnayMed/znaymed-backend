package db

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type TopicInfo struct {
	TgID        string
	Description string
	MindmapURL  string
}

func (d *Database) GetTopicInfoForUser(topicTitle string, userTgID string) (*TopicInfo, error) {
	var user User
	if err := d.DB.Where("tgid = ?", userTgID).First(&user).Error; err != nil {
		return nil, errors.New("пользователь не найден")
	}

	var topic Topic
	if err := d.DB.Where("title = ?", topicTitle).First(&topic).Error; err != nil {
		return nil, errors.New("тема не найдена")
	}

	var userSection UserSection
	err := d.DB.
		Where("user_id = ? AND section_id = ?", user.ID, topic.SectionID).
		First(&userSection).Error

	if err != nil {
		return nil, errors.New("у пользователя нет доступа к этой теме")
	}

	return &TopicInfo{
		TgID:        topic.TgID,
		Description: topic.Description,
		MindmapURL:  topic.MindmapURL,
	}, nil
}

func (d *Database) GetAccessibleSectionTitlesByTGIDHash(tgidHash string) ([]string, error) {
	var user User
	if err := d.DB.Where("tgid = ?", tgidHash).First(&user).Error; err != nil {
		return nil, err
	}
	var titles []string
	err := d.DB.
		Table("user_sections AS us").
		Select("s.title").
		Joins("JOIN sections s ON s.id = us.section_id").
		Where("us.user_id = ?", user.ID).
		Order("s.id").
		Scan(&titles).Error
	return titles, err
}

func (d *Database) GiveSectionToUser(tgid string, sectionTitle string) (bool, error) {
	var user User
	if err := d.DB.Where("tgid = ?", tgid).First(&user).Error; err != nil {
		return false, errors.New("пользователь не найден")
	}

	var section Section
	if err := d.DB.Where("title = ?", sectionTitle).First(&section).Error; err != nil {
		return false, errors.New("раздел не найден")
	}

	var count int64
	err := d.DB.Table("user_sections").
		Where("user_id = ? AND section_id = ?", user.ID, section.ID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, errors.New("раздел уже добавлен пользователю")
	}

	err = d.DB.Exec("INSERT INTO user_sections (user_id, section_id) VALUES (?, ?)", user.ID, section.ID).Error
	if err != nil {
		return false, err
	}

	return true, nil
}

func (d *Database) ListSubjects(ctx context.Context) ([]Subject, error) {
	var subjects []Subject
	err := d.DB.WithContext(ctx).
		Model(&Subject{}).
		Select("id", "title").
		Order("id").
		Find(&subjects).Error
	if err != nil {
		return nil, err
	}
	return subjects, nil
}

func (d *Database) GetSectionTitlesBySubjectTitle(subject string) ([]string, error) {
	var titles []string
	err := d.DB.
		Table("sections AS s").
		Select("s.title").
		Joins("JOIN subjects subj ON subj.id = s.subject_id").
		Where("subj.title = ?", subject).
		Order("s.id").
		Scan(&titles).Error
	return titles, err
}

func (d *Database) GetAccessibleSectionTitlesByTGIDAndSubject(tgid, subject string) ([]string, error) {
	var user User
	if err := d.DB.Where("tgid = ?", tgid).First(&user).Error; err != nil {
		return nil, err
	}
	var titles []string
	err := d.DB.
		Table("user_sections AS us").
		Select("s.title").
		Joins("JOIN sections s ON s.id = us.section_id").
		Joins("JOIN subjects subj ON subj.id = s.subject_id").
		Where("us.user_id = ? AND subj.title = ?", user.ID, subject).
		Order("s.id").
		Scan(&titles).Error
	return titles, err
}

func GetSectionIDByTitle(ctx context.Context, gdb *gorm.DB, title string) (uint, error) {
	var sec Section
	if err := gdb.WithContext(ctx).
		Select("id").
		Where("title = ?", title).
		First(&sec).Error; err != nil {
		return 0, err
	}
	return sec.ID, nil
}

func GetTopicsBySectionID(ctx context.Context, gdb *gorm.DB, sectionID uint) ([]Topic, error) {
	var topics []Topic
	if err := gdb.WithContext(ctx).
		Where("section_id = ?", sectionID).
		Order("id").
		Find(&topics).Error; err != nil {
		return nil, err
	}
	return topics, nil
}

func (d *Database) GetSectionPriceKopeckByTitle(ctx context.Context, sectionTitle string) (int64, error) {
	var price int64
	err := d.DB.WithContext(ctx).
		Table("sections").
		Where("title = ?", sectionTitle).
		Select("price_kopeck").
		Scan(&price).Error
	return price, err
}

func (d *Database) GetSectionDescriptionsBySubjectTitle(subject string) (map[string]string, error) {
	type row struct {
		Title       string
		Description string
	}
	var rows []row
	err := d.DB.
		Table("sections AS s").
		Select("s.title, s.description").
		Joins("JOIN subjects subj ON subj.id = s.subject_id").
		Where("subj.title = ?", subject).
		Order("s.id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Title] = r.Description
	}
	return out, nil
}
