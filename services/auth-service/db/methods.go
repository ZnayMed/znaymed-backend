package db

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
