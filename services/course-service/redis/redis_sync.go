package redis

import (
	"context"
	"fmt"

	coursedb "github.com/ZnayMed/znaymed-backend/services/course-service/db"
	redislib "github.com/redis/go-redis/v9"
)

// FillRedisFromDB выгружает все данные из БД в Redis,
// строго соблюдая схему ключей из FillData (никаких новых/других ключей).
// Синхронизацию user:<tgid>:sections НЕ выполняет.
func FillRedisFromDB(ctx context.Context, database *coursedb.Database, rdb *redislib.Client) error {
	// 1) Загружаем Subjects -> Sections -> Topics одной пачкой
	var subjects []coursedb.Subject
	if err := database.DB.WithContext(ctx).
		Preload("Sections.Topics").
		Find(&subjects).Error; err != nil {
		return fmt.Errorf("db preload subjects/sections/topics: %w", err)
	}

	pipe := rdb.Pipeline()
	cmds := 0
	flush := func() error {
		if cmds == 0 {
			return nil
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
		pipe = rdb.Pipeline()
		cmds = 0
		return nil
	}
	queue := func() {
		cmds++
		if cmds >= 1000 {
			_ = flush()
		}
	}

	// 2) Subjects + mappings + subject:<id>:sections
	for _, s := range subjects {
		// subject:<id>
		pipe.HSet(ctx, fmt.Sprintf("subject:%d", s.ID),
			"id", s.ID,
			"title", s.Title,
		)
		queue()

		// subject:title:<Title> -> <id>
		pipe.Set(ctx, "subject:title:"+s.Title, fmt.Sprintf("%d", s.ID), 0)
		queue()

		// subject:<id>:sections (пересобираем)
		secKey := fmt.Sprintf("subject:%d:sections", s.ID)
		pipe.Del(ctx, secKey)
		queue()
		if len(s.Sections) > 0 {
			ids := make([]interface{}, 0, len(s.Sections))
			for _, sec := range s.Sections {
				ids = append(ids, sec.ID)
			}
			pipe.SAdd(ctx, secKey, ids...)
			queue()
		}

		// 3) Sections + mappings + section:<id>:topics
		for _, sec := range s.Sections {
			// section:<id>
			pipe.HSet(ctx, fmt.Sprintf("section:%d", sec.ID),
				"id", sec.ID,
				"subject_id", s.ID,
				"title", sec.Title,
				"description", sec.Description,
				"price_kopeck", sec.PriceKopeck, // строго как в FillData
			)
			queue()

			// section:title:<Title> -> <id>
			pipe.Set(ctx, "section:title:"+sec.Title, fmt.Sprintf("%d", sec.ID), 0)
			queue()

			// section:<id>:topics (пересобираем)
			topKey := fmt.Sprintf("section:%d:topics", sec.ID)
			pipe.Del(ctx, topKey)
			queue()
			if len(sec.Topics) > 0 {
				tids := make([]interface{}, 0, len(sec.Topics))
				for _, t := range sec.Topics {
					tids = append(tids, t.ID)
				}
				pipe.SAdd(ctx, topKey, tids...)
				queue()
			}

			// 4) Topics
			for _, t := range sec.Topics {
				pipe.HSet(ctx, fmt.Sprintf("topic:%d", t.ID),
					"id", t.ID,
					"section_id", sec.ID,
					"title", t.Title,
					"description", t.Description,
					"tg_id", t.TgID,
					"mindmap_url", t.MindmapURL,
				)
				queue()
			}
		}
	}

	// финальный flush
	if err := flush(); err != nil {
		return fmt.Errorf("redis pipeline exec: %w", err)
	}

	return nil
}
