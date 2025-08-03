package db

import "errors"

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
func (d *Database) GetAccessibleSectionTitlesByTgID(tgid string) ([]string, error) {
	var user User

	if err := d.DB.Where("tgid = ?", tgid).First(&user).Error; err != nil {
		return nil, errors.New("пользователь не найден")
	}

	var titles []string
	err := d.DB.
		Table("sections").
		Select("sections.title").
		Joins("JOIN user_sections ON user_sections.section_id = sections.id").
		Where("user_sections.user_id = ?", user.ID).
		Scan(&titles).Error

	if err != nil {
		return nil, err
	}

	return titles, nil
}
