package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/course-service/db"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"log"
	"net"
)

type courseServer struct {
	pb.UnimplementedCourseServiceServer
	db *db.Database
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
func (s *courseServer) GetUserSections(ctx context.Context, req *pb.UserRequest) (*pb.UserSectionsResponse, error) {
	hashName := hashTGID(req.Tgid)
	titles, err := s.db.GetAccessibleSectionTitlesByTgID(hashName)
	if err != nil {
		log.Println("Ошибка при получении доступных разделов пользователя:", err)
		return &pb.UserSectionsResponse{TopicTitles: nil}, err
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
	StartKafkaConsumer(database)
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterCourseServiceServer(grpcServer, &courseServer{db: database})
	log.Println("CourseService listening on :50053")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
