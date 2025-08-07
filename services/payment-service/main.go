package main

import (
	"context"
	"github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/payment-service/db"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"log"
	"net"
	"strconv"
	"time"
)

type paymentServer struct {
	pb.UnimplementedPaymentServiceServer
	db *db.Database
}

func (s *paymentServer) startKafkaDispatcher(topic string) {
	go func() {
		writer := kafka.NewWriter(kafka.WriterConfig{
			Brokers: []string{"kafka:9092"},
			Topic:   topic,
		})
		defer writer.Close()

		for {
			time.Sleep(5 * time.Second)

			events, err := s.db.GetUnsentOutboxEvents()
			if err != nil {
				log.Println("❌ Ошибка при получении событий из outbox:", err)
				continue
			}

			for _, event := range events {
				err := writer.WriteMessages(context.Background(), kafka.Message{
					Key:   []byte(strconv.Itoa(int(event.ID))),
					Value: event.Payload,
				})
				if err != nil {
					log.Println("❌ Ошибка отправки в Kafka:", err)
					continue
				}

				err = s.db.MarkOutboxEventAsSent(event.ID)
				if err != nil {
					log.Println("❌ Ошибка пометки события как отправленного:", err)
				} else {
					log.Println("📤 Отправлено в Kafka:", event.ID)
				}
			}
		}
	}()
}

func (s *paymentServer) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	existing, err := s.db.FindPayment(req.Tgid, req.CourseId)
	if err == nil {
		log.Println("💰 Платёж уже существует")
		return &pb.CreatePaymentResponse{
			PaymentId:  existing.ID,
			PaymentUrl: "https://mock-psp/pay/" + existing.ID,
			Status:     existing.Status,
		}, nil
	}

	paymentID := uuid.NewString()
	newPayment := &db.Payment{
		ID:       paymentID,
		TgID:     req.Tgid,
		CourseID: req.CourseId,
		Status:   "PENDING",
	}

	if err := s.db.CreatePayment(newPayment); err != nil {
		log.Println("❌ Ошибка создания платежа:", err)
		return nil, err
	}

	log.Println("✅ Платёж создан:", paymentID)
	return &pb.CreatePaymentResponse{
		PaymentId:  paymentID,
		PaymentUrl: "https://mock-psp/pay/" + paymentID,
		Status:     "PENDING",
	}, nil
}

func main() {
	database, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}

	server := &paymentServer{db: database}
	server.startKafkaDispatcher("course-events")

	go StartCallbackHTTPServer(server)

	lis, err := net.Listen("tcp", ":50055")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPaymentServiceServer(grpcServer, server)

	log.Println("PaymentService listening on :50055")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
