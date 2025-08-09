package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

func New() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "redis:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
}

func FillData(ctx context.Context, rdb *redis.Client) {
	// ===== Предметы =====
	rdb.HSet(ctx, "subject:1", "id", 1, "title", "Математика")
	rdb.HSet(ctx, "subject:2", "id", 2, "title", "Физика")
	rdb.HSet(ctx, "subject:3", "id", 3, "title", "Химия")

	// Связи предмет → разделы
	rdb.SAdd(ctx, "subject:1:sections", 11, 12)
	rdb.SAdd(ctx, "subject:2:sections", 21, 22)
	rdb.SAdd(ctx, "subject:3:sections", 31, 32)

	// ===== Разделы =====
	// Математика
	rdb.HSet(ctx, "section:11", "id", 11, "subject_id", 1, "title", "Алгебра",
		"description", "Основы алгебры: выражения, уравнения, функции")
	rdb.Set(ctx, "section:title:Алгебра", 11, 0)

	rdb.HSet(ctx, "section:12", "id", 12, "subject_id", 1, "title", "Геометрия",
		"description", "Планиметрия и стереометрия")
	rdb.Set(ctx, "section:title:Геометрия", 12, 0)

	// Физика
	rdb.HSet(ctx, "section:21", "id", 21, "subject_id", 2, "title", "Механика",
		"description", "Законы Ньютона, кинематика, динамика")
	rdb.Set(ctx, "section:title:Механика", 21, 0)

	rdb.HSet(ctx, "section:22", "id", 22, "subject_id", 2, "title", "Оптика",
		"description", "Свет, линзы, зеркала")
	rdb.Set(ctx, "section:title:Оптика", 22, 0)

	// Химия
	rdb.HSet(ctx, "section:31", "id", 31, "subject_id", 3, "title", "Органическая химия",
		"description", "Углеводороды, функциональные группы")
	rdb.Set(ctx, "section:title:Органическая химия", 31, 0)

	rdb.HSet(ctx, "section:32", "id", 32, "subject_id", 3, "title", "Неорганическая химия",
		"description", "Соли, оксиды, кислоты")
	rdb.Set(ctx, "section:title:Неорганическая химия", 32, 0)

	// ===== Темы (связи раздел → темы + данные тем) =====
	// Алгебра
	rdb.SAdd(ctx, "section:11:topics", 111, 112)
	rdb.HSet(ctx, "topic:111", "id", 111, "section_id", 11, "title", "Линейные уравнения",
		"description", "ax + b = 0", "tg_id", "tg_math_lin", "mindmap_url", "https://mindmap.example.com/lin")
	rdb.HSet(ctx, "topic:112", "id", 112, "section_id", 11, "title", "Квадратные уравнения",
		"description", "ax² + bx + c = 0", "tg_id", "tg_math_quad", "mindmap_url", "https://mindmap.example.com/quad")

	// Геометрия
	rdb.SAdd(ctx, "section:12:topics", 121)
	rdb.HSet(ctx, "topic:121", "id", 121, "section_id", 12, "title", "Треугольники",
		"description", "Классификация и свойства", "tg_id", "tg_math_tri", "mindmap_url", "https://mindmap.example.com/tri")

	// Механика
	rdb.SAdd(ctx, "section:21:topics", 211)
	rdb.HSet(ctx, "topic:211", "id", 211, "section_id", 21, "title", "Третий закон Ньютона",
		"description", "F₁ = −F₂", "tg_id", "tg_phys_newton3", "mindmap_url", "https://mindmap.example.com/n3")

	// Оптика
	rdb.SAdd(ctx, "section:22:topics", 221)
	rdb.HSet(ctx, "topic:221", "id", 221, "section_id", 22, "title", "Линзы",
		"description", "Собирательные и рассеивающие", "tg_id", "tg_phys_lenses", "mindmap_url", "https://mindmap.example.com/lens")

	// Органическая химия
	rdb.SAdd(ctx, "section:31:topics", 311)
	rdb.HSet(ctx, "topic:311", "id", 311, "section_id", 31, "title", "Алканы",
		"description", "CnH₂n+2", "tg_id", "tg_chem_alkanes", "mindmap_url", "https://mindmap.example.com/alk")

	// Неорганическая химия
	rdb.SAdd(ctx, "section:32:topics", 321)
	rdb.HSet(ctx, "topic:321", "id", 321, "section_id", 32, "title", "Кислоты",
		"description", "Сильные и слабые", "tg_id", "tg_chem_acids", "mindmap_url", "https://mindmap.example.com/acid")

	// ===== Пользователи (пример покупок) =====
	rdb.SAdd(ctx, "user:1001:sections", 11, 12)
	rdb.SAdd(ctx, "user:1002:sections", 21)
	rdb.SAdd(ctx, "user:1003:sections", 31, 32)
}

func GetUserSectionTitles(ctx context.Context, rdb *redis.Client, tgid string) ([]string, error) {
	key := "user:" + tgid + ":sections"

	ids, err := rdb.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis SMEMBERS %s: %w", key, err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	titles := make([]string, 0, len(ids))
	for _, id := range ids {
		secKey := "section:" + id
		t, err := rdb.HGet(ctx, secKey, "title").Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("redis HGET %s title: %w", secKey, err)
		}
		if t != "" {
			titles = append(titles, t)
		}
	}

	if len(titles) == 0 {
		return nil, nil
	}

	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis TTL %s: %w", key, err)
	}

	if ttl > 0 {
		if err := rdb.Expire(ctx, key, 3*time.Hour).Err(); err != nil {
			return nil, fmt.Errorf("redis EXPIRE %s: %w", key, err)
		}
	}

	return titles, nil
}

func SaveUserSectionsByTitles(ctx context.Context, rdb *redis.Client, tgid string, titles []string, ttl time.Duration) error {
	if len(titles) == 0 {
		return nil
	}
	key := "user:" + tgid + ":sections"

	for _, title := range titles {
		idxKey := "section:title:" + title
		id, err := rdb.Get(ctx, idxKey).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return fmt.Errorf("redis GET %s: %w", idxKey, err)
		}
		if id == "" {
			continue
		}
		if err := rdb.SAdd(ctx, key, id).Err(); err != nil {
			return fmt.Errorf("redis SADD %s %s: %w", key, id, err)
		}
	}

	if err := rdb.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("redis EXPIRE %s: %w", key, err)
	}
	return nil
}
func GetSubjectTitles(ctx context.Context, rdb *redis.Client) ([]string, error) {
	ids, err := rdb.SMembers(ctx, "subjects:set").Result()
	if err != nil {
		return nil, fmt.Errorf("redis SMEMBERS subjects:set: %w", err)
	}
	if len(ids) == 0 {
		return []string{}, nil
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		title, err := rdb.HGet(ctx, "subject:"+id, "title").Result()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("redis HGET subject:%s title: %w", id, err)
		}
		if title != "" {
			out = append(out, title)
		}
	}
	return out, nil
}
