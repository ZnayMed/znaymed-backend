package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/course-service/db"
	rediscourse "github.com/ZnayMed/znaymed-backend/services/course-service/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type courseServer struct {
	pb.UnimplementedCourseServiceServer
	db  *db.Database
	rdb *goredis.Client
}
type paymentConfirmedEvent struct {
	Version    int      `json:"version"`
	PaymentID  string   `json:"payment_id"`
	Tgid       string   `json:"tgid"`
	CourseID   string   `json:"course_id"`
	CourseIDs  []string `json:"course_ids"`
	Amount     int64    `json:"amount"`
	Currency   string   `json:"currency"`
	ProviderID string   `json:"provider_id"`
	EventType  string   `json:"event_type"`
}

func (s *courseServer) GetTopicsBySectionTitle(ctx context.Context, req *pb.SectionTitleRequest) (*pb.SectionTopicsResponse, error) {
	title := req.SectionTitle
	if title == "" {
		log.Printf("GetTopicsBySectionTitle: empty section_title")
		return nil, status.Error(codes.InvalidArgument, "section_title is required")
	}

	log.Printf("GetTopicsBySectionTitle: section_title=%q", title)

	if s.rdb != nil {
		log.Printf("Trying Redis path for section_title=%q", title)

		sectionID, rerr := rediscourse.GetSectionIDByTitle(ctx, s.rdb, title)
		if rerr != nil {
			if rerr == goredis.Nil {
				log.Printf("Redis miss: section:title:%s not found", title)
			} else {
				log.Printf("Redis error in GetSectionIDByTitle: %v (will fallback to DB)", rerr)
			}
		} else {
			log.Printf("Redis sectionID=%s for title=%q", sectionID, title)

			topicIDs, rerr := rediscourse.GetSectionTopicIDs(ctx, s.rdb, sectionID)
			if rerr != nil {
				log.Printf("Redis error in GetSectionTopicIDs(%s): %v (will fallback to DB)", sectionID, rerr)
			} else {
				log.Printf("Redis topicIDs: %v", topicIDs)

				if len(topicIDs) == 0 {
					log.Printf("Redis: no topics for sectionID=%s (title=%q). Returning empty list (no DB fallback).", sectionID, title)
					return &pb.SectionTopicsResponse{Topics: nil}, nil
				}

				topics, rerr := rediscourse.GetTopicsByIDsPipeline(ctx, s.rdb, topicIDs)
				if rerr != nil {
					log.Printf("Redis error in GetTopicsByIDsPipeline: %v (will fallback to DB)", rerr)
				} else {
					log.Printf("Redis topics loaded: %d", len(topics))

					resp := &pb.SectionTopicsResponse{Topics: make([]*pb.TopicItem, 0, len(topics))}
					for _, t := range topics {
						resp.Topics = append(resp.Topics, &pb.TopicItem{
							Id:          t.ID,
							SectionId:   t.SectionID,
							Title:       t.Title,
							Description: t.Description,
							TgId:        t.TgID,
							MindmapUrl:  t.MindmapURL,
						})
					}
					return resp, nil
				}
			}
		}
	} else {
		log.Printf("Redis client is nil — using DB path")
	}

	log.Printf("Fallback to DB for section_title=%q", title)

	secID, err := db.GetSectionIDByTitle(ctx, s.db.DB, title)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("DB: section not found: %q", title)
			return &pb.SectionTopicsResponse{Topics: nil}, nil
		}
		log.Printf("DB error in dbGetSectionIDByTitle: %v", err)
		return nil, status.Errorf(codes.Internal, "db: section by title: %v", err)
	}

	topics, err := db.GetTopicsBySectionID(ctx, s.db.DB, secID)
	if err != nil {
		log.Printf("DB error in dbGetTopicsBySectionID: %v", err)
		return nil, status.Errorf(codes.Internal, "db: topics by section: %v", err)
	}

	log.Printf("DB topics loaded: %d (no Redis writes as requested)", len(topics))

	resp := &pb.SectionTopicsResponse{Topics: make([]*pb.TopicItem, 0, len(topics))}
	for _, t := range topics {
		resp.Topics = append(resp.Topics, &pb.TopicItem{
			Id:          int64(t.ID),
			SectionId:   int64(t.SectionID),
			Title:       t.Title,
			Description: t.Description,
			TgId:        t.TgID,
			MindmapUrl:  t.MindmapURL,
		})
	}
	return resp, nil
}

func AddSection(ctx context.Context, database *db.Database, rdb *goredis.Client, tgid string, title string) error {
	const userTTL = 3 * time.Hour
	hashName := hashTGID(tgid)
	log.Printf("📥 AddSectionFromKafka: hash=%s title=%s", tgid, title)

	success, err := database.GiveSectionToUser(hashName, title)
	if err != nil {
		log.Printf("DB GiveSectionToUser error: %v", err)
		return err
	}
	if !success {
		log.Printf("DB: nothing changed for hash=%s title=%s", hashName, title)
		return nil
	}

	exists, err := rediscourse.UserSectionsExists(ctx, rdb, tgid)
	if err != nil {
		log.Printf("Redis EXISTS user:%s:sections error: %v (skip warmup)", tgid, err)
		return nil
	}

	if exists {
		if err := rediscourse.AddUserSectionByTitle(ctx, rdb, tgid, title, userTTL); err != nil {
			log.Printf("Redis SADD user:%s:sections by title=%q failed: %v", tgid, title, err)
		} else {
			log.Printf("Redis updated: user:%s:sections += %q", tgid, title)
		}
	} else {
		titles, derr := database.GetAccessibleSectionTitlesByTGIDHash(hashName)
		if derr != nil {
			log.Printf("DB GetAccessibleSectionTitlesByTGIDHash error: %v (skip warmup)", derr)
			return nil
		}
		if err := rediscourse.SaveUserSectionsByTitles(ctx, rdb, tgid, titles, userTTL); err != nil {
			log.Printf("Redis warmup user:%s:sections failed: %v", hashName, err)
		} else {
			log.Printf("Redis warmed user:%s:sections with %d titles (TTL=%s)", hashName, len(titles), userTTL)
		}
	}

	return nil
}

func (s *courseServer) GetListSubjects(ctx context.Context, req *pb.ListSubjectsRequest) (*pb.ListSubjectsResponse, error) {
	if s.rdb != nil {
		if titles, err := rediscourse.GetSubjectTitles(ctx, s.rdb); err == nil && len(titles) > 0 {
			return &pb.ListSubjectsResponse{Titles: titles}, nil
		} else if err != nil {
			log.Printf("ListSubjects: Redis error: %v — fallback to DB", err)
		}
	}

	subjects, err := s.db.ListSubjects(ctx)
	if err != nil {
		return nil, err
	}
	titles := make([]string, 0, len(subjects))
	for _, sbj := range subjects {
		titles = append(titles, sbj.Title)
	}
	return &pb.ListSubjectsResponse{Titles: titles}, nil
}

func (s *courseServer) GetSubjectSections(ctx context.Context, req *pb.SubjectSectionsRequest) (*pb.SubjectSectionsResponse, error) {
	const userTTL = 3 * time.Hour
	start := time.Now()

	log.Printf("GetSubjectSections: subject='%s', tgid=%s", req.Subject, req.Tgid)

	subjectID, err := rediscourse.GetSubjectIDByTitle(ctx, s.rdb, req.Subject)
	if err != nil {
		log.Printf("Redis GetSubjectIDByTitle('%s') error: %v", req.Subject, err)
	}
	if subjectID == "" {
		log.Printf("Redis: subject id for '%s' not found", req.Subject)
	}

	subIDs, err := rediscourse.GetSubjectSectionIDs(ctx, s.rdb, subjectID)
	if err != nil {
		log.Printf("Redis GetSubjectSectionIDs(subjectID=%s) error: %v", subjectID, err)
	}
	if len(subIDs) == 0 {
		log.Printf("Redis: no section IDs for subjectID=%s", subjectID)
	}

	id2title, miss, err := rediscourse.GetSectionTitlesByIDs(ctx, s.rdb, subIDs)
	if err != nil {
		log.Printf("Redis GetSectionTitlesByIDs error: %v", err)
	}
	if miss > 0 {
		log.Printf("Redis: missing %d section titles (subjectID=%s)", miss, subjectID)
	}

	hasFullSubjectInRedis := subjectID != "" && len(subIDs) > 0 && miss == 0
	if hasFullSubjectInRedis {
		userIDs, err := rediscourse.GetUserSectionIDs(ctx, s.rdb, req.Tgid)
		if err != nil {
			log.Printf(" Redis GetUserSectionIDs(tgid=%s) error: %v", req.Tgid, err)
		}

		if len(userIDs) == 0 {
			log.Printf("Redis: no user sections for tgid=%s — fallback DB", req.Tgid)

			accTitles, dberr := s.db.GetAccessibleSectionTitlesByTGIDAndSubject(hashTGID(req.Tgid), req.Subject)
			if dberr != nil {
				log.Printf("DB GetAccessibleSectionTitlesByTGIDAndSubject error: %v", dberr)
				accTitles = nil
			}

			if len(accTitles) > 0 {
				if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, accTitles, userTTL); err != nil {
					log.Printf("Redis SaveUserSectionsByTitles error: %v", err)
				} else {
					log.Printf("Redis warmed user:%s:sections with %d titles (TTL=%s)", req.Tgid, len(accTitles), userTTL)
				}
			} else {
				log.Printf("DB: no accessible titles for tgid=%s, subject='%s'", req.Tgid, req.Subject)
			}

			accTitleSet := make(map[string]struct{}, len(accTitles))
			for _, t := range accTitles {
				accTitleSet[t] = struct{}{}
			}

			resp := &pb.SubjectSectionsResponse{Sections: make([]*pb.SectionItem, 0, len(subIDs))}
			for _, id := range subIDs {
				title := id2title[id]
				_, ok := accTitleSet[title]
				resp.Sections = append(resp.Sections, &pb.SectionItem{
					Title:      title,
					Accessible: ok,
				})
			}
			log.Printf("GetSubjectSections OK (redis subject + db user) in %s", time.Since(start))
			return resp, nil
		}

		accIDs := make(map[string]struct{}, len(userIDs))
		for _, id := range userIDs {
			accIDs[id] = struct{}{}
		}

		resp := &pb.SubjectSectionsResponse{Sections: make([]*pb.SectionItem, 0, len(subIDs))}
		for _, id := range subIDs {
			_, ok := accIDs[id]
			resp.Sections = append(resp.Sections, &pb.SectionItem{
				Title:      id2title[id],
				Accessible: ok,
			})
		}
		log.Printf("GetSubjectSections OK (redis only) in %s", time.Since(start))
		return resp, nil
	}

	log.Printf("Fallback to DB for subject='%s'", req.Subject)

	allTitles, err := s.db.GetSectionTitlesBySubjectTitle(req.Subject)
	if err != nil {
		log.Printf(" DB GetSectionTitlesBySubjectTitle error: %v", err)
		return nil, status.Errorf(codes.Internal, "db: sections by subject: %v", err)
	}
	accTitles, err := s.db.GetAccessibleSectionTitlesByTGIDAndSubject(hashTGID(req.Tgid), req.Subject)
	if err != nil {
		log.Printf("DB GetAccessibleSectionTitlesByTGIDAndSubject error: %v", err)
		accTitles = nil
	}

	if len(accTitles) > 0 {
		if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, accTitles, userTTL); err != nil {
			log.Printf("Redis SaveUserSectionsByTitles error: %v", err)
		} else {
			log.Printf("Redis warmed user:%s:sections with %d titles (TTL=%s)", req.Tgid, len(accTitles), userTTL)
		}
	}

	accSet := make(map[string]struct{}, len(accTitles))
	for _, t := range accTitles {
		accSet[t] = struct{}{}
	}

	resp := &pb.SubjectSectionsResponse{Sections: make([]*pb.SectionItem, 0, len(allTitles))}
	for _, title := range allTitles {
		_, ok := accSet[title]
		resp.Sections = append(resp.Sections, &pb.SectionItem{
			Title:      title,
			Accessible: ok,
		})
	}
	log.Printf("GetSubjectSections OK (db fallback) in %s", time.Since(start))
	return resp, nil
}

func hashTGID(tgid string) string {
	hash := sha256.Sum256([]byte(tgid))
	return hex.EncodeToString(hash[:])
}

func extractCourseIDs(courseIDField string) []string {
	if strings.HasPrefix(courseIDField, "MULTI:") {
		rest := strings.TrimPrefix(courseIDField, "MULTI:")
		if rest == "" {
			return nil
		}
		parts := strings.Split(rest, "|")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}

	id := strings.TrimSpace(courseIDField)
	if id == "" {
		return nil
	}
	return []string{id}
}

func StartKafkaConsumer(database *db.Database, ctx context.Context, rdb *goredis.Client) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"kafka:9092"},
		Topic:    "course-events",
		GroupID:  "course-consumer-group",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	go func() {
		log.Println("Kafka Consumer started for topic: course-events")
		for {
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Kafka read error: %v", err)
				continue
			}

			var evt paymentConfirmedEvent
			if err := json.Unmarshal(msg.Value, &evt); err != nil {
				log.Printf("Ошибка при разборе события: %v; raw=%s", err, string(msg.Value))
				continue
			}

			ids := evt.CourseIDs
			if len(ids) == 0 {
				ids = extractCourseIDs(evt.CourseID)
			}
			if len(ids) == 0 {
				log.Printf("Пустой список курсов в событии payment_id=%s", evt.PaymentID)
				continue
			}

			for _, cid := range ids {
				if err := AddSection(ctx, database, rdb, evt.Tgid, cid); err != nil {
					log.Printf("Ошибка при добавлении курса '%s' пользователю '%s': %v", cid, evt.Tgid, err)
					continue
				}
				log.Printf("Курс %s добавлен пользователю %s (payment=%s)", cid, evt.Tgid, evt.PaymentID)
			}
		}
	}()
}

func setDiffStr(all, owned []string) []string {
	if len(all) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(owned))
	for _, s := range owned {
		m[s] = struct{}{}
	}
	out := make([]string, 0, len(all))
	for _, s := range all {
		if _, ok := m[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

func (s *courseServer) MissingSectionsBySubjects(ctx context.Context, in *pb.MissingSectionsRequest) (*pb.MissingSectionsResponse, error) {
	if in == nil || in.Tgid == "" || len(in.Subjects) == 0 {
		return nil, status.Error(codes.InvalidArgument, "tgid and subjects are required")
	}
	out := &pb.MissingSectionsResponse{Result: make(map[string]*pb.SubjectMissing, len(in.Subjects))}

	for _, subj := range in.Subjects {
		var allTitles []string
		subjectID, _ := rediscourse.GetSubjectIDByTitle(ctx, s.rdb, subj)
		if subjectID != "" {
			secIDs, _ := rediscourse.GetSubjectSectionIDs(ctx, s.rdb, subjectID)
			if len(secIDs) > 0 {
				if id2title, miss, err := rediscourse.GetSectionTitlesByIDs(ctx, s.rdb, secIDs); err == nil && miss == 0 {
					allTitles = make([]string, 0, len(secIDs))
					for _, id := range secIDs {
						allTitles = append(allTitles, id2title[id])
					}
				}
			}
		}

		if len(allTitles) == 0 {
			titles, err := s.db.GetSectionTitlesBySubjectTitle(subj)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "db sections by subject: %v", err)
			}
			allTitles = titles
		}

		ownedTitles, err := s.db.GetAccessibleSectionTitlesByTGIDAndSubject(hashTGID(in.Tgid), subj)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "db user owned by subject: %v", err)
		}

		missing := setDiffStr(allTitles, ownedTitles)
		out.Result[subj] = &pb.SubjectMissing{SectionIds: missing}
	}
	return out, nil
}

func (s *courseServer) SubjectMissingTotal(ctx context.Context, in *pb.SubjectMissingTotalRequest) (*pb.SubjectMissingTotalResponse, error) {
	if in == nil || in.Tgid == "" || in.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "tgid and subject are required")
	}

	var allTitles []string
	if subjectID, _ := rediscourse.GetSubjectIDByTitle(ctx, s.rdb, in.Subject); subjectID != "" {
		if secIDs, _ := rediscourse.GetSubjectSectionIDs(ctx, s.rdb, subjectID); len(secIDs) > 0 {
			if id2title, miss, err := rediscourse.GetSectionTitlesByIDs(ctx, s.rdb, secIDs); err == nil && miss == 0 {
				allTitles = make([]string, 0, len(secIDs))
				for _, id := range secIDs {
					allTitles = append(allTitles, id2title[id])
				}
			}
		}
	}
	if len(allTitles) == 0 {
		titles, err := s.db.GetSectionTitlesBySubjectTitle(in.Subject)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "db sections by subject: %v", err)
		}
		allTitles = titles
	}

	ownedTitles, err := s.db.GetAccessibleSectionTitlesByTGIDAndSubject(hashTGID(in.Tgid), in.Subject)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "db user owned by subject: %v", err)
	}

	missing := setDiffStr(allTitles, ownedTitles)

	var total int64
	for _, title := range missing {
		if v, ok := rediscourse.GetSectionPrice(ctx, s.rdb, in.Subject, title); ok {
			total += v
			continue
		}
		priceK, derr := s.db.GetSectionPriceKopeckByTitle(ctx, title)
		if derr != nil {
			return nil, status.Errorf(codes.Internal, "db price by title: %v", derr)
		}
		total += priceK
		rediscourse.SetSectionPrice(ctx, s.rdb, in.Subject, title, priceK)
	}

	return &pb.SubjectMissingTotalResponse{
		Subject:     in.Subject,
		TotalKopeck: total,
		Currency:    "RUB",
	}, nil
}

func setOf(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func (s *courseServer) PriceMissingFromList(ctx context.Context, in *pb.PriceMissingRequest) (*pb.PriceMissingResponse, error) {
	if in == nil || in.Tgid == "" || len(in.Sections) == 0 {
		return nil, status.Error(codes.InvalidArgument, "tgid and sections are required")
	}

	ownedTitles, err := s.db.GetAccessibleSectionTitlesByTGIDHash(hashTGID(in.Tgid))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "db owned titles: %v", err)
	}
	owned := setOf(ownedTitles)

	missing := make([]string, 0, len(in.Sections))
	for _, title := range in.Sections {
		if _, ok := owned[title]; !ok {
			missing = append(missing, title)
		}
	}
	if len(missing) == 0 {
		return &pb.PriceMissingResponse{
			MissingSections: nil,
			TotalKopeck:     0,
			Currency:        "RUB",
		}, nil
	}

	var total int64
	for _, title := range missing {
		secID, _ := rediscourse.GetSectionIDByTitle(ctx, s.rdb, title) // может вернуть redis.Nil
		var price int64
		var ok bool
		if secID != "" {
			if p, err := s.rdb.HGet(ctx, "section:"+secID, "price_kopeck").Result(); err == nil && p != "" {
				if v, conv := strconv.ParseInt(p, 10, 64); conv == nil {
					price, ok = v, true
				}
			}
		}
		if !ok {
			v, derr := s.db.GetSectionPriceKopeckByTitle(ctx, title)
			if derr != nil {
				return nil, status.Errorf(codes.Internal, "db price by title %q: %v", title, derr)
			}
			price = v
		}
		total += price
	}

	return &pb.PriceMissingResponse{
		MissingSections: missing,
		TotalKopeck:     total,
		Currency:        "RUB",
	}, nil
}

func main() {
	database, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}

	//redis
	ctx := context.Background()
	rdb := rediscourse.New()

	if err := rediscourse.FillRedisFromDB(ctx, database, rdb); err != nil {
		log.Fatalf("fill redis from db failed: %v", err)
	}

	//kafka
	StartKafkaConsumer(database, ctx, rdb)

	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterCourseServiceServer(grpcServer, &courseServer{
		db:  database,
		rdb: rdb,
	})
	log.Println("CourseService listening on :50053")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
