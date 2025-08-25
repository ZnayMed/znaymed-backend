package main

import (
	"context"
	"fmt"
	"github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/payment-service/db"
	"github.com/ZnayMed/znaymed-backend/services/payment-service/provider"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
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
				log.Println("Ошибка при получении событий из outbox:", err)
				continue
			}

			for _, event := range events {
				err := writer.WriteMessages(context.Background(), kafka.Message{
					Key:   []byte(strconv.Itoa(int(event.ID))),
					Value: event.Payload,
				})
				if err != nil {
					log.Println("Ошибка отправки в Kafka:", err)
					continue
				}

				err = s.db.MarkOutboxEventAsSent(event.ID)
				if err != nil {
					log.Println("Ошибка пометки события как отправленного:", err)
				} else {
					log.Println("Отправлено в Kafka:", event.ID)
				}
			}
		}
	}()
}

func getCoursePriceRUB(ctx context.Context, courseID string) (amountInKopecks int64, err error) {
	return 19900, nil
}

func (s *paymentServer) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	if p, err := s.db.FindActivePending(req.Tgid, req.CourseId); err == nil {
		log.Println("Уже есть активный платеж, возвращаю ConfirmationURL")
		return &pb.CreatePaymentResponse{
			PaymentId:  p.ID,
			PaymentUrl: p.ConfirmationURL,
			Status:     p.Status,
		}, nil
	}

	amountK, err := getCoursePriceRUB(ctx, req.CourseId)
	if err != nil {
		return nil, err
	}
	amountRubStr := fmt.Sprintf("%.2f", float64(amountK)/100.0)

	paymentID := uuid.NewString()
	idemKey := uuid.NewString()

	p := &db.Payment{
		ID:             paymentID,
		TgID:           req.Tgid,
		CourseID:       req.CourseId,
		Status:         "PENDING",
		Amount:         amountK,
		Currency:       "RUB",
		Description:    "Оплата доступа к курсу/разделу",
		Provider:       "yookassa",
		IdempotenceKey: idemKey,
	}
	if err := s.db.CreatePayment(p); err != nil {
		log.Println("Ошибка создания платежа:", err)
		return nil, err
	}

	yoo := provider.NewYooKassa()
	returnURL := os.Getenv("PUBLIC_RETURN_URL")
	providerID, confirmationURL, status, expiresAt, err := yoo.CreatePayment(ctx, amountRubStr, p.Description, returnURL, idemKey, map[string]string{
		"internal_payment_id": p.ID,
		"tgid":                p.TgID,
		"course_id":           p.CourseID,
	})
	if err != nil {
		log.Println("YooKassa CreatePayment:", err)
		return nil, err
	}

	tx := s.db.DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	if err := s.db.UpdateProviderFields(tx, p.ID, providerID, confirmationURL); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if expiresAt != nil {
		if err := tx.Model(&db.Payment{}).Where("id = ?", p.ID).
			Update("expires_at", *expiresAt).Error; err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	return &pb.CreatePaymentResponse{
		PaymentId:  p.ID,
		PaymentUrl: confirmationURL,
		Status:     statusToLocal(status),
	}, nil
}

func statusToLocal(yoo string) string {
	switch yoo {
	case "succeeded":
		return "PAID"
	case "canceled":
		return "CANCELED"
	default:
		return "PENDING"
	}
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
