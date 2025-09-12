package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
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
		DB:          0,
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

	rdb.Set(ctx, "subject:title:Математика", 1, 0)
	rdb.Set(ctx, "subject:title:Физика", 2, 0)
	rdb.Set(ctx, "subject:title:Химия", 3, 0)

	// ===== Разделы =====
	// Математика
	rdb.HSet(ctx, "section:11",
		"id", 11, "subject_id", 1, "title", "Алгебра",
		"description", "Основы алгебры: выражения, уравнения, функции",
		"price_kopeck", 19900,
	)
	rdb.Set(ctx, "section:title:Алгебра", 11, 0)

	rdb.HSet(ctx, "section:12",
		"id", 12, "subject_id", 1, "title", "Геометрия",
		"description", "Планиметрия и стереометрия",
		"price_kopeck", 24900,
	)
	rdb.Set(ctx, "section:title:Геометрия", 12, 0)

	// Физика
	rdb.HSet(ctx, "section:21",
		"id", 21, "subject_id", 2, "title", "Механика",
		"description", "Законы Ньютона, кинематика, динамика",
		"price_kopeck", 29900,
	)
	rdb.Set(ctx, "section:title:Механика", 21, 0)

	rdb.HSet(ctx, "section:22",
		"id", 22, "subject_id", 2, "title", "Оптика",
		"description", "Свет, линзы, зеркала",
		"price_kopeck", 15900,
	)
	rdb.Set(ctx, "section:title:Оптика", 22, 0)

	// Химия
	rdb.HSet(ctx, "section:31",
		"id", 31, "subject_id", 3, "title", "Органическая химия",
		"description", "Углеводороды, функциональные группы",
		"price_kopeck", 18900,
	)
	rdb.Set(ctx, "section:title:Органическая химия", 31, 0)

	rdb.HSet(ctx, "section:32",
		"id", 32, "subject_id", 3, "title", "Неорганическая химия",
		"description", "Соли, оксиды, кислоты",
		"price_kopeck", 17900,
	)
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

func GetSubjectSectionIDs(ctx context.Context, rdb *redis.Client, subjectID string) ([]string, error) {
	if subjectID == "" {
		return nil, nil
	}
	key := "subject:" + subjectID + ":sections"
	ids, err := rdb.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("SMEMBERS %s: %w", key, err)
	}
	ttl, err := rdb.TTL(ctx, key).Result()
	if err == nil && ttl > 0 {
		_ = rdb.Persist(ctx, key).Err()
	}
	return ids, nil
}

func GetSectionIDByTitle(ctx context.Context, rdb *redis.Client, title string) (string, error) {
	key := "section:title:" + title
	return rdb.Get(ctx, key).Result()
}

func GetSectionTitlesByIDs(ctx context.Context, rdb *redis.Client, ids []string) (map[string]string, int, error) {
	m := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return m, 0, nil
	}
	pipe := rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGet(ctx, "section:"+id, "title")
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, 0, fmt.Errorf("pipeline HGET title: %w", err)
	}

	miss := 0
	for i, id := range ids {
		val, e := cmds[i].Result()
		if e == redis.Nil || val == "" {
			miss++
			continue
		}
		if e != nil {
			miss++
			continue
		}
		if ttl, e2 := rdb.TTL(ctx, "section:"+id).Result(); e2 == nil && ttl > 0 {
			_ = rdb.Persist(ctx, "section:"+id).Err()
		}
		m[id] = val
	}
	return m, miss, nil
}

func GetSubjectIDByTitle(ctx context.Context, rdb *redis.Client, title string) (string, error) {
	if title == "" {
		return "", nil
	}
	key := "subject:title:" + title
	id, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", key, err)
	}
	if ttl, e := rdb.TTL(ctx, key).Result(); e == nil && ttl > 0 {
		_ = rdb.Persist(ctx, key).Err()
	}
	return id, nil
}

func GetUserSectionIDs(ctx context.Context, rdb *redis.Client, tgid string) ([]string, error) {
	key := "user:" + tgid + ":sections"
	ids, err := rdb.SMembers(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("SMEMBERS %s: %w", key, err)
	}
	_ = rdb.Expire(ctx, key, 2*time.Minute).Err()
	return ids, nil
}

func SaveUserSectionsByTitles(ctx context.Context, rdb *redis.Client, tgid string, titles []string, ttl time.Duration) error {
	if len(titles) == 0 {
		return nil
	}
	key := "user:" + tgid + ":sections"

	pipe := rdb.Pipeline()
	for _, title := range titles {
		idxKey := "section:title:" + title
		id, err := rdb.Get(ctx, idxKey).Result()
		if err == redis.Nil || id == "" {
			continue
		}
		if err != nil {
			return fmt.Errorf("GET %s: %w", idxKey, err)
		}
		pipe.SAdd(ctx, key, id)
	}
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline SADD/EXPIRE %s: %w", key, err)
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

func UserSectionsExists(ctx context.Context, rdb *redis.Client, tgid string) (bool, error) {
	key := "user:" + tgid + ":sections"
	n, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func AddUserSectionByTitle(ctx context.Context, rdb *redis.Client, tgid, title string, ttl time.Duration) error {
	if title == "" || tgid == "" {
		return fmt.Errorf("empty tgid or title")
	}
	id, err := rdb.Get(ctx, "section:title:"+title).Result()
	if err == redis.Nil || id == "" {
		return fmt.Errorf("section id not found for title %q", title)
	}
	if err != nil {
		return fmt.Errorf("GET section:title:%s: %w", title, err)
	}
	key := "user:" + tgid + ":sections"
	pipe := rdb.Pipeline()
	pipe.SAdd(ctx, key, id)
	pipe.Expire(ctx, key, ttl)
	_, execErr := pipe.Exec(ctx)
	return execErr
}

func GetSectionTopicIDs(ctx context.Context, rdb *redis.Client, sectionID string) ([]string, error) {
	key := "section:" + sectionID + ":topics"
	return rdb.SMembers(ctx, key).Result()
}

type Topic struct {
	ID          int64
	SectionID   int64
	Title       string
	Description string
	TgID        string
	MindmapURL  string
}

func GetTopicByID(ctx context.Context, rdb *redis.Client, topicID string) (*Topic, error) {
	key := "topic:" + topicID

	m, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("HGetAll %s: %w", key, err)
	}
	if len(m) == 0 {
		return nil, redis.Nil
	}

	id, _ := strconv.ParseInt(m["id"], 10, 64)
	secID, _ := strconv.ParseInt(m["section_id"], 10, 64)

	return &Topic{
		ID:          id,
		SectionID:   secID,
		Title:       m["title"],
		Description: m["description"],
		TgID:        m["tg_id"],
		MindmapURL:  m["mindmap_url"],
	}, nil
}

func GetTopicsByIDsPipeline(ctx context.Context, rdb *redis.Client, topicIDs []string) ([]*Topic, error) {
	if len(topicIDs) == 0 {
		return nil, nil
	}

	pipe := rdb.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, 0, len(topicIDs))
	for _, id := range topicIDs {
		cmds = append(cmds, pipe.HGetAll(ctx, "topic:"+id))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("pipeline HGetAll topics: %w", err)
	}

	out := make([]*Topic, 0, len(topicIDs))
	for i, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil {
			// пропустим отсутствующий топик
			continue
		}
		if len(m) == 0 {
			continue
		}
		id, _ := strconv.ParseInt(m["id"], 10, 64)
		secID, _ := strconv.ParseInt(m["section_id"], 10, 64)
		out = append(out, &Topic{
			ID:          id,
			SectionID:   secID,
			Title:       m["title"],
			Description: m["description"],
			TgID:        m["tg_id"],
			MindmapURL:  m["mindmap_url"],
		})
		_ = i
	}
	return out, nil
}

func keySectionPrice(subject, sectionTitle string) string {
	return "section_price:" + subject + ":" + sectionTitle
}

func GetSectionPrice(ctx context.Context, rdb *redis.Client, subject, sectionTitle string) (int64, bool) {
	s, err := rdb.Get(ctx, keySectionPrice(subject, sectionTitle)).Result()
	if err != nil || s == "" {
		return 0, false
	}
	v, convErr := strconv.ParseInt(s, 10, 64)
	if convErr != nil {
		return 0, false
	}
	return v, true
}

func SetSectionPrice(ctx context.Context, rdb *redis.Client, subject, sectionTitle string, priceK int64) {
	_ = rdb.Set(ctx, keySectionPrice(subject, sectionTitle), strconv.FormatInt(priceK, 10), 10*time.Minute).Err()
}

func GetSectionDescriptionsByIDs(ctx context.Context, rdb *redis.Client, ids []string) (map[string]string, int, error) {
	m := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return m, 0, nil
	}
	pipe := rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGet(ctx, "section:"+id, "description")
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, 0, fmt.Errorf("pipeline HGET description: %w", err)
	}

	miss := 0
	for i, id := range ids {
		val, e := cmds[i].Result()
		if e == redis.Nil || val == "" {
			miss++
			continue
		}
		if e != nil {
			miss++
			continue
		}
		if ttl, e2 := rdb.TTL(ctx, "section:"+id).Result(); e2 == nil && ttl > 0 {
			_ = rdb.Persist(ctx, "section:"+id).Err()
		}
		m[id] = val
	}
	return m, miss, nil
}

type SubjectInfoRedis struct {
	Title       string
	Description string
}

func GetSubjectInfos(ctx context.Context, rdb *redis.Client) ([]SubjectInfoRedis, error) {
	ids, err := rdb.SMembers(ctx, "subjects:set").Result()
	if err != nil {
		return nil, fmt.Errorf("redis SMEMBERS subjects:set: %w", err)
	}
	if len(ids) == 0 {
		return []SubjectInfoRedis{}, nil
	}

	pipe := rdb.Pipeline()
	titleCmds := make([]*redis.StringCmd, 0, len(ids))
	descCmds := make([]*redis.StringCmd, 0, len(ids))
	for _, id := range ids {
		titleCmds = append(titleCmds, pipe.HGet(ctx, "subject:"+id, "title"))
		descCmds = append(descCmds, pipe.HGet(ctx, "subject:"+id, "description"))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("pipeline HGET subject fields: %w", err)
	}

	out := make([]SubjectInfoRedis, 0, len(ids))
	for i, id := range ids {
		title, _ := titleCmds[i].Result()
		desc, _ := descCmds[i].Result()
		if title == "" {
			continue
		}
		if ttl, e := rdb.TTL(ctx, "subject:"+id).Result(); e == nil && ttl > 0 {
			_ = rdb.Persist(ctx, "subject:"+id).Err()
		}
		out = append(out, SubjectInfoRedis{
			Title:       title,
			Description: desc,
		})
	}
	return out, nil
}
