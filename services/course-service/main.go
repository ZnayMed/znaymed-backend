package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ZnayMed/znaymed-backend/pb"
	"github.com/ZnayMed/znaymed-backend/services/course-service/db"
	"google.golang.org/grpc"
	"log"
	"net"
)

type courseServer struct {
	pb.UnimplementedCourseServiceServer
	db *db.Database
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

func main() {
	database, err := db.NewDatabase()
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
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
