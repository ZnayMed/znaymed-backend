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
	hashName := hashTGID(req.Tgid)
	success, err := s.db.GiveSectionToUser(hashName, req.Title)
	if err != nil {
		log.Println("Ошибка при сохранении раздела для пользователя:", req.Tgid)
		return &pb.SaveSectionResponse{Success: success}, err
	}
	return &pb.SaveSectionResponse{Success: success}, err

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

func (s *courseServer) GetUserSections(ctx context.Context, req *pb.UserRequest) (*pb.UserSectionsResponse, error) {
	log.Printf("📥 Запрос разделов для пользователя TGID=%s", req.Tgid)

	titles, rerr := rediscourse.GetUserSectionTitles(ctx, s.rdb, req.Tgid)
	if rerr != nil {
		log.Printf("⚠️ Redis ошибка: %v (упадём в БД)", rerr)
	}
	if len(titles) > 0 {
		log.Printf("✅ Найдено в Redis: %v", titles)
		return &pb.UserSectionsResponse{TopicTitles: titles}, nil
	}
	log.Println("⚠️ В Redis нет данных, обращаемся к БД")

	hashName := hashTGID(req.Tgid)
	titles, err := s.db.GetAccessibleSectionTitlesByTgID(hashName)
	if err != nil {
		log.Printf("❌ Ошибка при получении из БД: %v", err)
		return &pb.UserSectionsResponse{TopicTitles: nil}, err
	}
	log.Printf("✅ Найдено в БД: %v", titles)

	if len(titles) > 0 {
		if err := rediscourse.SaveUserSectionsByTitles(ctx, s.rdb, req.Tgid, titles, 3*time.Hour); err != nil {
			log.Printf("⚠️ Не удалось сохранить в Redis: %v", err)
		} else {
			log.Printf("💾 Сохранено в Redis (TTL 3h): %v", titles)
		}
	} else {
		log.Println("⚠️ В БД нет доступных разделов")
	}

	return &pb.UserSectionsResponse{TopicTitles: titles}, nil
}

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
