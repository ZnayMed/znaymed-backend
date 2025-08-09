package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/course-service/db"
	rediscourse "github.com/ZnayMed/znaymed-backend/services/course-service/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"time"
)

type courseServer struct {
	pb.UnimplementedCourseServiceServer
	db  *db.Database
	rdb *goredis.Client
}

func (s *courseServer) AddSection(ctx context.Context, req *pb.SaveSectionRequest) (*pb.SaveSectionResponse, error) {
	const userTTL = 3 * time.Hour

	hashName := hashTGID(req.Tgid)
	log.Printf("📥 AddSection: tgid=%s (hash=%s) title=%s", req.Tgid, hashName, req.Title)

	// 1) Пишем в БД
	success, err := s.db.GiveSectionToUser(hashName, req.Title)
	if err != nil {
		log.Printf("❌ DB GiveSectionToUser error: %v", err)
		return &pb.SaveSectionResponse{Success: false}, err
	}
	if !success {
		log.Printf("ℹ️ DB: nothing changed for tgid=%s title=%s", req.Tgid, req.Title)
		return &pb.SaveSectionResponse{Success: false}, nil
	}

	// 2) Обновляем Redis
	exists, err := rediscourse.UserSectionsExists(ctx, s.rdb, req.Tgid)
	if err != nil {
		log.Printf("⚠️ Redis EXISTS user:%s:sections error: %v (skip warmup)", req.Tgid, err)
		return &pb.SaveSectionResponse{Success: true}, nil
	}

	if exists {
		// Ключ есть — добавляем только новый раздел
		if err := rediscourse.AddUserSectionByTitle(ctx, s.rdb, req.Tgid, req.Title, userTTL); err != nil {
			log.Printf("⚠️ Redis SADD user:%s:sections by title=%q failed: %v", req.Tgid, req.Title, err)
		} else {
			log.Printf("💾 Redis updated: user:%s:sections += %q", req.Tgid, req.Title)
		}
	} else {
		// Ключа нет — гидратируем ПОЛНЫЙ набор доступных разделов из БД (включая только что добавленный)
		titles, derr := s.db.GetAccessibleSectionTitlesByTGIDHash(hashName)
		if derr != nil {
			log.Printf("⚠️ DB GetAccessibleSectionTitlesByTGIDHash error: %v (skip warmup)", derr)
			return &pb.SaveSectionResponse{Success: true}, nil
		}
		if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, titles, userTTL); err != nil {
			log.Printf("⚠️ Redis warmup user:%s:sections failed: %v", req.Tgid, err)
		} else {
			log.Printf("💾 Redis warmed user:%s:sections with %d titles (TTL=%s)", req.Tgid, len(titles), userTTL)
		}
	}

	return &pb.SaveSectionResponse{Success: true}, nil
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

	log.Printf("📥 GetSubjectSections: subject='%s', tgid=%s", req.Subject, req.Tgid)

	subjectID, err := rediscourse.GetSubjectIDByTitle(ctx, s.rdb, req.Subject)
	if err != nil {
		log.Printf("⚠️ Redis GetSubjectIDByTitle('%s') error: %v", req.Subject, err)
	}
	if subjectID == "" {
		log.Printf("ℹ️ Redis: subject id for '%s' not found", req.Subject)
	}

	subIDs, err := rediscourse.GetSubjectSectionIDs(ctx, s.rdb, subjectID)
	if err != nil {
		log.Printf("⚠️ Redis GetSubjectSectionIDs(subjectID=%s) error: %v", subjectID, err)
	}
	if len(subIDs) == 0 {
		log.Printf("ℹ️ Redis: no section IDs for subjectID=%s", subjectID)
	}

	id2title, miss, err := rediscourse.GetSectionTitlesByIDs(ctx, s.rdb, subIDs)
	if err != nil {
		log.Printf("⚠️ Redis GetSectionTitlesByIDs error: %v", err)
	}
	if miss > 0 {
		log.Printf("ℹ️ Redis: missing %d section titles (subjectID=%s)", miss, subjectID)
	}

	hasFullSubjectInRedis := subjectID != "" && len(subIDs) > 0 && miss == 0
	if hasFullSubjectInRedis {
		userIDs, err := rediscourse.GetUserSectionIDs(ctx, s.rdb, req.Tgid)
		if err != nil {
			log.Printf("⚠️ Redis GetUserSectionIDs(tgid=%s) error: %v", req.Tgid, err)
		}

		if len(userIDs) == 0 {
			log.Printf("ℹ️ Redis: no user sections for tgid=%s — fallback DB", req.Tgid)

			accTitles, dberr := s.db.GetAccessibleSectionTitlesByTGIDAndSubject(req.Tgid, req.Subject)
			if dberr != nil {
				log.Printf("⚠️ DB GetAccessibleSectionTitlesByTGIDAndSubject error: %v", dberr)
				accTitles = nil
			}

			if len(accTitles) > 0 {
				if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, accTitles, userTTL); err != nil {
					log.Printf("⚠️ Redis SaveUserSectionsByTitles error: %v", err)
				} else {
					log.Printf("💾 Redis warmed user:%s:sections with %d titles (TTL=%s)", req.Tgid, len(accTitles), userTTL)
				}
			} else {
				log.Printf("ℹ️ DB: no accessible titles for tgid=%s, subject='%s'", req.Tgid, req.Subject)
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
			log.Printf("✅ GetSubjectSections OK (redis subject + db user) in %s", time.Since(start))
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
		log.Printf("✅ GetSubjectSections OK (redis only) in %s", time.Since(start))
		return resp, nil
	}

	log.Printf("↪️ Fallback to DB for subject='%s'", req.Subject)

	allTitles, err := s.db.GetSectionTitlesBySubjectTitle(req.Subject)
	if err != nil {
		log.Printf("❌ DB GetSectionTitlesBySubjectTitle error: %v", err)
		return nil, status.Errorf(codes.Internal, "db: sections by subject: %v", err)
	}
	accTitles, err := s.db.GetAccessibleSectionTitlesByTGIDAndSubject(req.Tgid, req.Subject)
	if err != nil {
		log.Printf("⚠️ DB GetAccessibleSectionTitlesByTGIDAndSubject error: %v", err)
		accTitles = nil
	}

	if len(accTitles) > 0 {
		if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, accTitles, userTTL); err != nil {
			log.Printf("⚠️ Redis SaveUserSectionsByTitles error: %v", err)
		} else {
			log.Printf("💾 Redis warmed user:%s:sections with %d titles (TTL=%s)", req.Tgid, len(accTitles), userTTL)
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
	log.Printf("✅ GetSubjectSections OK (db fallback) in %s", time.Since(start))
	return resp, nil
}

//func (s *courseServer) GetUserSections(ctx context.Context, req *pb.UserRequest) (*pb.UserSectionsResponse, error) {
//	log.Printf("📥 Запрос разделов для пользователя TGID=%s", req.Tgid)
//
//	titles, rerr := rediscourse.GetUserSectionTitles(ctx, s.rdb, req.Tgid)
//	if rerr != nil {
//		log.Printf("⚠️ Redis ошибка: %v (упадём в БД)", rerr)
//	}
//	if len(titles) > 0 {
//		log.Printf("✅ Найдено в Redis: %v", titles)
//		return &pb.UserSectionsResponse{TopicTitles: titles}, nil
//	}
//	log.Println("⚠️ В Redis нет данных, обращаемся к БД")
//
//	hashName := hashTGID(req.Tgid)
//	titles, err := s.db.GetAccessibleSectionTitlesByTgID(hashName)
//	if err != nil {
//		log.Printf("❌ Ошибка при получении из БД: %v", err)
//		return &pb.UserSectionsResponse{TopicTitles: nil}, err
//	}
//	log.Printf("✅ Найдено в БД: %v", titles)
//
//	if len(titles) > 0 {
//		if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, titles, 3*time.Hour); err != nil {
//			log.Printf("⚠️ Не удалось сохранить в Redis: %v", err)
//		} else {
//			log.Printf("💾 Сохранено в Redis (TTL 3h): %v", titles)
//		}
//	} else {
//		log.Println("⚠️ В БД нет доступных разделов")
//	}
//
//	return &pb.UserSectionsResponse{TopicTitles: titles}, nil
//}

func hashTGID(tgid string) string {
	hash := sha256.Sum256([]byte(tgid))
	return hex.EncodeToString(hash[:])
}

func StartKafkaConsumer(database *db.Database) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"kafka:9092"},
		Topic:    "course-events",
		GroupID:  "course-consumer-group",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	go func() {
		log.Println("📥 Kafka Consumer started for topic: course-events")
		for {
			m, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("❌ Kafka read error: %v", err)
				continue
			}

			log.Printf("📨 Получено сообщение: %s", string(m.Value))

			var event struct {
				PaymentID string `json:"payment_id"`
				Tgid      string `json:"tgid"`
				CourseID  string `json:"course_id"`
			}
			if err := json.Unmarshal(m.Value, &event); err != nil {
				log.Printf("❌ Ошибка при разборе события: %v", err)
				continue
			}

			_, err = database.GiveSectionToUser(hashTGID(event.Tgid), event.CourseID)
			if err != nil {
				log.Printf("❌ Ошибка при добавлении курса пользователю: %v", err)
			} else {
				log.Printf("✅ Курс %s добавлен пользователю %s", event.CourseID, event.Tgid)
			}
		}
	}()
}

func main() {
	database, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}

	//kafka
	StartKafkaConsumer(database)

	//redis
	ctx := context.Background()
	rdb := rediscourse.New()
	rediscourse.FillData(ctx, rdb)

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
